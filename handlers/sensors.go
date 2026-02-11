package handlers

import (
	"encoding/json"
	"go-system-monitor/model"
	"log/slog"
	"net/http"
	"os/exec"
)

func SensorsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	output, err := exec.Command("sensors", "-j").Output()
	if err != nil {
		slog.Info("Error running sensors command", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:   "Failed to retrieve sensor data",
			Details: err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(output)
}
