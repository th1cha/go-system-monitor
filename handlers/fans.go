package handlers

import (
	"encoding/json"
	"fmt"
	"go-system-monitor/model"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const hwmonBase = "/sys/class/hwmon"

func readIntFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	return v, nil
}

func readOptionalInt(path string) int {
	v, _ := readIntFile(path)
	return v
}

func writeIntFile(path string, value int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(value)+"\n"), 0644)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string, details ...string) {
	resp := model.ErrorResponse{Error: msg}
	if len(details) > 0 {
		resp.Details = details[0]
	}
	writeJSON(w, status, resp)
}

func hwmonName(hwmonDir string) string {
	data, err := os.ReadFile(filepath.Join(hwmonDir, "name"))
	if err != nil {
		return filepath.Base(hwmonDir)
	}
	return strings.TrimSpace(string(data))
}

func readLabel(dir string, pwmIdx int) string {
	data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("fan%d_label", pwmIdx)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func pwmMode(enableVal int) string {
	if enableVal == 1 {
		return "manual"
	}
	return "auto"
}

func pwmMaxVal(dir string, pwmIdx int) int {
	v := readOptionalInt(filepath.Join(dir, fmt.Sprintf("pwm%d_max", pwmIdx)))
	if v == 0 {
		return 255
	}
	return v
}

func buildFan(dir string, pwmIdx int) (model.Fan, bool) {
	pwmPath := filepath.Join(dir, fmt.Sprintf("pwm%d", pwmIdx))
	pwm, err := readIntFile(pwmPath)
	if err != nil {
		return model.Fan{}, false
	}

	maxVal := pwmMaxVal(dir, pwmIdx)
	rpm := readOptionalInt(filepath.Join(dir, fmt.Sprintf("fan%d_input", pwmIdx)))
	enableVal := readOptionalInt(filepath.Join(dir, fmt.Sprintf("pwm%d_enable", pwmIdx)))
	alarm := readOptionalInt(filepath.Join(dir, fmt.Sprintf("fan%d_alarm", pwmIdx)))

	return model.Fan{
		PwmIndex:   pwmIdx,
		Label:      readLabel(dir, pwmIdx),
		Active:     rpm > 0,
		RPM:        rpm,
		PWMRaw:     pwm,
		PWMPercent: pwm * 100 / maxVal,
		PWMMin:     readOptionalInt(filepath.Join(dir, fmt.Sprintf("pwm%d_min", pwmIdx))),
		PWMMax:     maxVal,
		Mode:       pwmMode(enableVal),
		Alarm:      alarm == 1,
	}, true
}

func discoverFansInDir(dir string) []model.Fan {
	pwmFiles, _ := filepath.Glob(filepath.Join(dir, "pwm[0-9]*"))
	var fans []model.Fan
	for _, pwmPath := range pwmFiles {
		base := filepath.Base(pwmPath)
		if strings.Contains(base, "_") {
			continue
		}
		pwmIdx, err := strconv.Atoi(strings.TrimPrefix(base, "pwm"))
		if err != nil {
			continue
		}
		if fan, ok := buildFan(dir, pwmIdx); ok {
			fans = append(fans, fan)
		}
	}
	return fans
}

func discoverChips() ([]model.Chip, error) {
	entries, err := os.ReadDir(hwmonBase)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", hwmonBase, err)
	}

	var chips []model.Chip
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "hwmon") {
			continue
		}
		hwmonIdx, err := strconv.Atoi(strings.TrimPrefix(name, "hwmon"))
		if err != nil {
			continue
		}
		dir := filepath.Join(hwmonBase, name)
		fans := discoverFansInDir(dir)
		if len(fans) == 0 {
			continue
		}
		chips = append(chips, model.Chip{
			HwmonIndex: hwmonIdx,
			HwmonName:  hwmonName(dir),
			Fans:       fans,
		})
	}
	return chips, nil
}

func resolveFanPaths(hwmonIndex, pwmIndex int) (dir, pwmPath, enablePath string, err error) {
	dir = filepath.Join(hwmonBase, fmt.Sprintf("hwmon%d", hwmonIndex))
	if _, err = os.Stat(dir); err != nil {
		return "", "", "", fmt.Errorf("hwmon%d not found", hwmonIndex)
	}
	pwmPath = filepath.Join(dir, fmt.Sprintf("pwm%d", pwmIndex))
	if _, err = os.Stat(pwmPath); err != nil {
		return "", "", "", fmt.Errorf("pwm%d not found on hwmon%d", pwmIndex, hwmonIndex)
	}
	enablePath = filepath.Join(dir, fmt.Sprintf("pwm%d_enable", pwmIndex))
	return dir, pwmPath, enablePath, nil
}

func normalizeMode(mode string) (string, error) {
	mode = strings.ToLower(mode)
	if mode == "" {
		mode = "manual"
	}
	if mode != "manual" && mode != "auto" {
		return "", fmt.Errorf(`mode must be "manual" or "auto"`)
	}
	return mode, nil
}

func calcPWMValue(req model.SetFanSpeedRequest, maxVal int) (int, error) {
	if req.PWMRaw != nil {
		return *req.PWMRaw, nil
	}
	if req.PWMPercent != nil {
		return *req.PWMPercent * maxVal / 100, nil
	}
	return -1, fmt.Errorf("pwmRaw (0-%d) or pwmPercent (0-100) is required in manual mode", maxVal)
}

func applyManualPWM(dir string, req model.SetFanSpeedRequest) error {
	maxVal := pwmMaxVal(dir, req.PwmIndex)
	pwmVal, err := calcPWMValue(req, maxVal)
	if err != nil {
		return err
	}
	if pwmVal < 0 || pwmVal > maxVal {
		return fmt.Errorf("pwmRaw must be 0-%d or pwmPercent must be 0-100", maxVal)
	}
	return writeIntFile(filepath.Join(dir, fmt.Sprintf("pwm%d", req.PwmIndex)), pwmVal)
}

// GetFansHandler handles GET /api/fans and returns all fan chips and their current state.
func GetFansHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	chips, err := discoverChips()
	if err != nil {
		slog.Error("Failed to discover fans", "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to read fan data", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.FansResponse{Chips: chips})
}

// SetFanSpeedHandler handles PUT /api/fans/speed and sets the speed of a specific fan.
func SetFanSpeedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req model.SetFanSpeedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	dir, _, enablePath, err := resolveFanPaths(req.HwmonIndex, req.PwmIndex)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	mode, err := normalizeMode(req.Mode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	enableVal := map[string]int{"manual": 1, "auto": 2}[mode]
	if err := writeIntFile(enablePath, enableVal); err != nil {
		slog.Error("Failed to set fan mode", "path", enablePath, "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to set fan mode", err.Error())
		return
	}

	if mode == "manual" {
		if err := applyManualPWM(dir, req); err != nil {
			slog.Error("Failed to set fan speed", "error", err)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	fan, ok := buildFan(dir, req.PwmIndex)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Failed to read back fan state")
		return
	}

	writeJSON(w, http.StatusOK, model.SetFanSpeedResponse{
		HwmonIndex: req.HwmonIndex,
		HwmonName:  hwmonName(dir),
		Fan:        fan,
	})
}
