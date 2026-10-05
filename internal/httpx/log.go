package httpx

import "log/slog"

func WriteErrLog(msg, method string, errMsg string, err error) {
	slog.Error(msg, "method", method, errMsg, err.Error())
}
