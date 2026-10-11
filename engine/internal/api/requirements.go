package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/setup"
	"github.com/gemgum/clipper/engine/internal/transcribe"
)

// Halaman Requirements: status komponen + pemasangannya.
//
// Dua endpoint, sesuai rencana di notes/23:
//
//	GET  /api/requirements          daftar komponen + statusnya
//	POST /api/requirements/install  memasang satu komponen, mengalirkan progres
//
// Progresnya memakai format SSE yang sama dengan job — satu mekanisme aliran
// untuk seluruh engine, bukan dua.

// requirements melaporkan status semua komponen.
func (s *Server) requirements(w http.ResponseWriter, r *http.Request) {
	// Letak program dibaca ulang tiap kali halaman ini bertanya. "Check again"
	// karenanya benar-benar memeriksa ulang SEGALANYA — termasuk program yang
	// baru dipasang di luar aplikasi (mis. Ollama, atau ffmpeg lewat winget) —
	// bukan cuma membaca ulang jawaban yang sama.
	s.applyPaths()

	// Ollama diperiksa lewat jaringan; itu bagian paling lambat di halaman ini,
	// tapi tanpa hasilnya barisnya cuma bisa bilang "tidak tahu".
	oll := ollama.Status(r.Context(), r.URL.Query().Get("ollama_url"))
	detail := ""
	switch {
	case oll.Running && len(oll.Installed) > 0:
		var names []string
		for _, m := range oll.Installed {
			names = append(names, m.Base)
		}
		detail = "Running. Models: " + strings.Join(names, ", ")
	case oll.Running:
		detail = "Running, but no model is installed yet: pull one below."
	}

	as := transcribe.AIStudioFromEnv()
	comps := setup.Status(s.layout)
	// Server LLM bukan berkas yang engine pasang, jadi ia tidak punya path. Yang
	// setara — dan yang justru dicari orang saat susunannya Windows+WSL — adalah
	// ALAMAT dan DI SISTEM MANA ia berjalan.
	//
	// Hanya SATU baris yang ditandai berjalan: engine memakai satu server per
	// job, dan menandai dua baris hijau akan menyiratkan keduanya terpakai.
	for i := range comps {
		if oll.Running && comps[i].ID == "llm:"+llmID(oll.Server) {
			comps[i].Installed = true
			comps[i].Name = comps[i].Name + " · " + oll.Where
			comps[i].Path = oll.URL
			if detail != "" {
				comps[i].Detail = detail
			}
		}
	}
	writeJSON(w, 200, map[string]any{
		"components": comps,
		"missing":    setup.Missing(comps),
		"data_dir":   s.layout.DataDir,
		"models_dir": s.layout.ModelsDir,
		"tools_dir":  s.layout.ToolsDir,
		"dev":        s.layout.Dev,
		"ollama":     oll,
		// Transkripsi AI Studio (notes/43): cukup tahu kuncinya ada atau tidak,
		// dan model mana yang akan dipakai. Kuncinya sendiri tidak pernah dikirim.
		"ai_studio": map[string]any{
			"key_set": as.Key != "",
			"model":   as.Model,
		},
	})
}

// removeComponent menghapus model yang sudah diunduh.
//
// Ada karena model besar memakan 2,9 GB dan pengguna yang mencobanya sekali
// harus punya cara membuangnya tanpa mencari-cari foldernya sendiri.
func (s *Server) removeComponent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	name, ok := strings.CutPrefix(strings.TrimSpace(req.ID), "model:")
	if !ok {
		writeErr(w, 400, "only whisper models can be removed from here")
		return
	}
	if err := setup.RemoveModel(s.layout, name); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"ok":         true,
		"components": setup.Status(s.layout),
	})
}

// setComponentPath menyimpan letak sebuah program yang ditunjuk pengguna.
//
// Ada karena mendeteksi otomatis tidak akan pernah menang melawan semua cara
// orang memasang program. Sampai kini jalan keluarnya adalah menyuruh pengguna
// menyunting berkas .env — instruksi yang masuk akal untuk CLI dan tidak masuk
// akal sama sekali untuk aplikasi berjendela.
//
// Disimpan ke .env supaya bertahan setelah aplikasi ditutup, DAN dipasang ke
// proses yang sedang jalan supaya langsung terpakai tanpa perlu memulai ulang.
func (s *Server) setComponentPath(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	req.Path = hostPath(req.Path)
	if req.Path == "" {
		writeErr(w, 400, "the 'path' field is required")
		return
	}
	st, err := os.Stat(req.Path)
	if err != nil || st.IsDir() {
		writeErr(w, 400, "that file does not exist: "+req.Path)
		return
	}

	key := ""
	switch req.ID {
	case "chrome":
		key = "CLIPPER_CHROME"
	case "ffmpeg":
		key = "CLIPPER_FFMPEG_BIN"
	case "ffprobe":
		key = "CLIPPER_FFPROBE_BIN"
	case "whisper":
		key = "CLIPPER_WHISPER_BIN"
	default:
		writeErr(w, 400, "that component cannot be pointed at a file")
		return
	}

	if err := writeEnvKey(s.paths.EnvFile, key, req.Path); err != nil {
		writeErr(w, 500, "cannot save the setting: "+err.Error())
		return
	}
	// Env proses ikut diubah: ResolvePaths & capture.Find membacanya, jadi job
	// berikutnya memakai pilihan ini tanpa aplikasi ditutup dulu.
	_ = os.Setenv(key, req.Path)
	s.applyPaths()

	writeJSON(w, 200, map[string]any{
		"ok":         true,
		"components": setup.Status(s.layout),
	})
}

// listModels melaporkan model whisper yang tersedia/terunduh.
//
// Bertahan demi GUI yang sudah ada; daftar modelnya sendiri sekarang tinggal
// satu, di paket setup.
func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	type modelInfo struct {
		Name       string `json:"name"`
		Size       string `json:"size"`
		Downloaded bool   `json:"downloaded"`
	}
	out := make([]modelInfo, 0, len(setup.Models))
	for _, m := range setup.Models {
		out = append(out, modelInfo{
			Name: m.Name, Size: m.Size,
			Downloaded: setup.HasModel(s.layout, m.Name),
		})
	}
	writeJSON(w, 200, out)
}

// requirementsGuard menolak job bila komponen wajib belum ada, dengan menyebut
// yang mana. Tanpa ini pesannya baru muncul dari ffmpeg belasan detik kemudian,
// dalam bahasa yang tidak menyebut apa yang harus dipasang.
func (s *Server) missingRequirement() error {
	missing := setup.Missing(setup.Status(s.layout))
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s is not installed yet: open the Requirements page to install it",
		strings.Join(missing, " and "))
}

// llmID memetakan nama server yang terdeteksi kembali ke ID barisnya di
// setup.LLMServers. Dipisah supaya kedua daftar tidak perlu memakai ejaan yang
// sama persis ("llama.cpp" bukan ID yang sah).
func llmID(server string) string {
	switch server {
	case "Ollama":
		return "ollama"
	case "LM Studio":
		return "lmstudio"
	case "Jan":
		return "jan"
	case "llama.cpp":
		return "llamacpp"
	case "KoboldCpp":
		return "koboldcpp"
	case "GPT4All":
		return "gpt4all"
	}
	return ""
}
