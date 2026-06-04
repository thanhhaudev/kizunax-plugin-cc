package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/config"
	xerrors "github.com/thanhhaudev/llmreviewkit/errors"
	"github.com/thanhhaudev/llmreviewkit/provider"
)

// webIdleTimeout is how long the server waits for the user to submit before giving up.
const webIdleTimeout = 5 * time.Minute

// webPostGrace is how long the server stays alive after a successful save so the
// browser tab can render the success page before the process exits.
const webPostGrace = 2 * time.Second

// formData drives the HTML template.
type formData struct {
	Token            string
	Error            string
	Rotation         string
	OpenAIModel      string
	AnthropicModel   string
	ExistingKeyCount int
}

// setupWeb is the user-facing entry point: it picks a free port, generates a
// one-shot token, kills any prior worker, prints the URL, opens the user's
// browser, then spawns a detached worker child to actually serve the form.
// Returns sub-100ms — the worker holds the server until save or timeout.
func setupWeb() error {
	noOpen := false
	for _, a := range os.Args {
		if a == "--no-open" {
			noOpen = true
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return xerrors.Internal("listen", "cannot bind to 127.0.0.1", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		_ = ln.Close()
		return xerrors.Internal("rand", "cannot generate token", err)
	}
	token := hex.EncodeToString(tokenBytes)

	_ = killOldSetupWebWorker()

	url := fmt.Sprintf("http://127.0.0.1:%d/?t=%s", port, token)
	idleExit := time.Now().Add(webIdleTimeout)
	fmt.Println("Open this in your browser to finish setup:")
	fmt.Println(url)
	fmt.Printf("Worker idle-exits at %s unless you save first.\n", idleExit.Format("15:04:05"))

	if !noOpen {
		openInBrowser(url)
	}

	pid, err := spawnSetupWebWorker(ln, token)
	// Parent never serves on the listener — close its own ref so only the worker holds it.
	_ = ln.Close()
	if err != nil {
		return err
	}

	startedAt := idleExit.Add(-webIdleTimeout)
	if err := writeSetupWebState(setupWebState{
		PID:          pid,
		StartedAt:    startedAt,
		IdleDeadline: idleExit,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot write state file: %v\n", err)
	}
	return nil
}

// serveSetupWeb runs the HTTP handlers against ln (created by the parent and
// inherited from fd 3 in the worker). Returns when a save completes, the
// idle timeout fires, or a signal arrives.
func serveSetupWeb(ln net.Listener, token string) error {
	formTmpl, err := template.New("form").Parse(setupFormHTML)
	if err != nil {
		return xerrors.Internal("parse_template", "cannot parse form template", err)
	}

	saved := make(chan struct{}, 1)
	var savedFlag atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") != token {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		renderForm(w, formTmpl, token, "", loadInitialFormData())
	})
	mux.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") != token {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if savedFlag.Load() {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(setupSuccessHTML))
			return
		}
		fd, errMsg := parseForm(r.PostForm)
		fd.Token = token
		if errMsg != "" {
			fd.Error = errMsg
			renderForm(w, formTmpl, token, errMsg, fd)
			return
		}
		if err := writeConfigFromForm(fd, r.PostForm); err != nil {
			fd.Error = err.Error()
			renderForm(w, formTmpl, token, err.Error(), fd)
			return
		}
		savedFlag.Store(true)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(setupSuccessHTML))
		select {
		case saved <- struct{}{}:
		default:
		}
	})
	mux.HandleFunc("/list-models", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") != token {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			writeListModelsError(w, "bad form")
			return
		}
		prov := r.URL.Query().Get("provider")
		apiKey := strings.TrimSpace(r.PostForm.Get("key"))
		if apiKey == "" {
			writeListModelsError(w, "paste at least one API key first")
			return
		}
		var baseURL string
		switch prov {
		case "openai":
			baseURL = config.KizunaXOpenAIBaseURL
		case "anthropic":
			baseURL = config.KizunaXAnthropicBaseURL
		case "helper":
			baseURL = config.KizunaXHelperBaseURL
			// Use the saved helper base_url if the config file has one.
			if f, err := config.LoadFile(); err == nil && f.Helper != nil && f.Helper.BaseURL != "" {
				baseURL = f.Helper.BaseURL
			}
		default:
			writeListModelsError(w, "unknown provider")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		models, err := provider.ListModels(ctx, baseURL, apiKey)
		if err != nil {
			if errors.Is(err, provider.ErrNoListModels) {
				writeListModelsFallback(w, prov)
				return
			}
			writeListModelsError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"models": models})
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-saved:
		time.Sleep(webPostGrace)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		configPath, _ := config.Path()
		_ = writeSetupWebResult(setupWebResult{
			Outcome:     setupWebSuccess,
			Message:     "configuration saved",
			CompletedAt: time.Now(),
			ConfigPath:  configPath,
		})
		return nil
	case <-time.After(webIdleTimeout):
		_ = srv.Close()
		_ = writeSetupWebResult(setupWebResult{
			Outcome:     setupWebTimeout,
			Message:     "5 min idle without save",
			CompletedAt: time.Now(),
		})
		return xerrors.User("timeout", "setup timed out without a save", "")
	case <-sigCh:
		_ = srv.Close()
		_ = writeSetupWebResult(setupWebResult{
			Outcome:     setupWebCancelled,
			Message:     "signal received",
			CompletedAt: time.Now(),
		})
		return xerrors.User("cancelled", "setup cancelled by signal", "")
	}
}

// loadInitialFormData reads the current config (if any) and returns initial form values.
func loadInitialFormData() formData {
	fd := formData{
		Rotation:       config.RotationRoundRobin,
		OpenAIModel:    config.DefaultOpenAIModel,
		AnthropicModel: config.DefaultAnthropicModel,
	}
	file, err := config.LoadFile()
	if err != nil {
		return fd
	}
	file = config.MigrateLegacy(file)
	if file.Rotation != "" {
		fd.Rotation = file.Rotation
	}
	if file.OpenAIModel != "" {
		fd.OpenAIModel = file.OpenAIModel
	}
	if file.AnthropicModel != "" {
		fd.AnthropicModel = file.AnthropicModel
	}
	fd.ExistingKeyCount = len(file.APIKeys)
	return fd
}

// renderForm renders the form template. errMsg goes into the banner (empty = no banner).
func renderForm(w http.ResponseWriter, tmpl *template.Template, token, errMsg string, fd formData) {
	fd.Token = token
	fd.Error = errMsg
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, fd)
}

// parseForm reads the form values into a formData (preserving them for re-render)
// and returns the first validation error (if any). It does NOT include API keys
// in the returned formData — keys are pulled separately during write.
func parseForm(values url.Values) (formData, string) {
	rotation := strings.TrimSpace(values.Get("rotation"))
	if rotation == "" {
		rotation = config.RotationRoundRobin
	}
	if rotation != config.RotationRoundRobin {
		return formData{Rotation: config.RotationRoundRobin},
			fmt.Sprintf("rotation %q not supported in this release.", rotation)
	}
	// Models may be absent (disabled <select> doesn't submit) — leave them empty
	// and let writeConfigFromForm fall back to existing config → built-in default.
	fd := formData{
		Rotation:       rotation,
		OpenAIModel:    strings.TrimSpace(values.Get("openai_model")),
		AnthropicModel: strings.TrimSpace(values.Get("anthropic_model")),
	}
	return fd, ""
}

// parseKeysTextarea splits the textarea contents by newlines, trims each line,
// drops blanks, dedupes while preserving order.
func parseKeysTextarea(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range strings.Split(s, "\n") {
		k := strings.TrimSpace(raw)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// writeConfigFromForm reads the (already-validated) form, merges with existing
// config (reusing keys where the form left the key field blank), and saves.
func writeConfigFromForm(fd formData, values url.Values) error {
	existing, _ := config.LoadFile()
	existing = config.MigrateLegacy(existing)

	keys := parseKeysTextarea(values.Get("api_keys"))
	if len(keys) == 0 {
		keys = existing.APIKeys
	}
	if len(keys) == 0 {
		return errors.New("At least one API key is required.")
	}

	openaiModel := fd.OpenAIModel
	if openaiModel == "" {
		openaiModel = existing.OpenAIModel
	}
	if openaiModel == "" {
		openaiModel = config.DefaultOpenAIModel
	}
	anthropicModel := fd.AnthropicModel
	if anthropicModel == "" {
		anthropicModel = existing.AnthropicModel
	}
	if anthropicModel == "" {
		anthropicModel = config.DefaultAnthropicModel
	}

	out := config.File{
		APIKeys:        keys,
		Rotation:       fd.Rotation,
		OpenAIModel:    openaiModel,
		AnthropicModel: anthropicModel,
		Temperature:    existing.Temperature,
		MaxTokens:      existing.MaxTokens,
	}

	// Parse helper block fields.
	helperBaseURL := strings.TrimSpace(values.Get("helper_base_url"))
	helperModel := strings.TrimSpace(values.Get("helper_model"))
	helperKeysRaw := values.Get("helper_api_keys")
	helperTimeoutStr := strings.TrimSpace(values.Get("helper_timeout_seconds"))

	helperKeys := parseKeysTextarea(helperKeysRaw)
	helperTimeout := 0
	if helperTimeoutStr != "" {
		if n, perr := strconv.Atoi(helperTimeoutStr); perr == nil && n > 0 {
			helperTimeout = n
		}
	}

	hasHelperConfig := helperBaseURL != "" || helperModel != "" || len(helperKeys) > 0 || helperTimeout > 0
	if hasHelperConfig {
		out.Helper = &config.HelperConfigFile{
			BaseURL:        helperBaseURL,
			Model:          helperModel,
			APIKeys:        helperKeys,
			TimeoutSeconds: helperTimeout,
		}
	} else if existing.Helper != nil {
		// Preserve existing helper config if the form didn't supply anything.
		out.Helper = existing.Helper
	}

	if err := config.Save(out); err != nil {
		return fmt.Errorf("cannot save config: %v", err)
	}
	return nil
}

func writeListModelsError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeListModelsFallback(w http.ResponseWriter, prov string) {
	var fallback []string
	switch prov {
	case "openai":
		fallback = []string{config.DefaultOpenAIModel, "coding/kimi-k2.6"}
	case "anthropic":
		fallback = []string{config.DefaultAnthropicModel, "MiniMax-M2.5-highspeed"}
	case "helper":
		fallback = []string{config.DefaultHelperModel}
	}
	writeJSON(w, map[string]any{
		"fallback": fallback,
		"note":     "upstream does not list models — using built-in defaults.",
	})
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payload)
}
