package usecase

import (
	"encoding/json"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// ComputeSuccessRate loads all events from the event store and returns
// the success rate, clean count, and total check count.
func ComputeSuccessRate(store port.EventStore) (rate float64, clean int, total int, err error) {
	events, loadErr := store.LoadAll()
	if loadErr != nil {
		return 0, 0, 0, loadErr
	}
	if len(events) == 0 {
		return 0, 0, 0, nil
	}

	rate = domain.SuccessRate(events)
	for _, ev := range events {
		if ev.Type != domain.EventCheckCompleted {
			continue
		}
		var data domain.CheckCompletedData
		if jsonErr := json.Unmarshal(ev.Data, &data); jsonErr != nil {
			continue
		}
		total++
		if len(data.Result.DMails) == 0 {
			clean++
		}
	}
	return rate, clean, total, nil
}
