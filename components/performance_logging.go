package components

import (
	"context"
	"errors"
	"time"

	"github.com/jorgerojas26/lazysql/helpers/logger"
)

// logDatabaseOperation emits the stable fields used by local performance
// diagnostics. The logger intentionally receives no SQL or row values.
func logDatabaseOperation(operation string, started time.Time, ctx context.Context, fields map[string]any, err error) {
	data := make(map[string]any, len(fields)+4)
	for key, value := range fields {
		data[key] = value
	}

	if err != nil {
		data["error"] = err.Error()
	}
	if ctxErr := contextError(ctx, err); ctxErr != nil {
		data["cancelled"] = true
		if _, present := data["cancellation_reason"]; !present {
			data["cancellation_reason"] = cancellationReason(ctxErr)
		}
	}

	logger.DebugOperation(operation, started, data)
}

func logFirstUsefulResult(surface string, started time.Time, fields map[string]any) {
	logger.DebugFirstUsefulResult(surface, started, fields)
}

func contextError(ctx context.Context, err error) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}

func cancellationReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline"
	}
	return "cancelled"
}
