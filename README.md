# Go System Monitor

A small HTTP server that monitors sensors and system information via API.


## API Endpoints
- GET /api/sensors - get system sensor data via lm-sensors.
- GET /api/fans - list all fan chips and their current RPM, PWM, and mode.
- PUT /api/fans/speed - set a fan's speed (pwm_raw or pwm_percent) or switch between manual/auto mode.