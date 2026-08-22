package savegame

import (
	"strings"
	"time"
)

const saveDateTimeLayout = "2006.01.02-15.04.05"

// PickLatestAutosave returns the newest autosave header for activeSessionName.
func PickLatestAutosave(activeSessionName string, sessions []Session) (SaveHeader, error) {
	active := strings.TrimSpace(activeSessionName)
	if active == "" {
		return SaveHeader{}, ErrNoActiveSave
	}

	var headers []SaveHeader
	for _, sess := range sessions {
		if strings.TrimSpace(sess.SessionName) != active {
			continue
		}
		headers = append(headers, sess.SaveHeaders...)
		break
	}
	if len(headers) == 0 {
		return SaveHeader{}, ErrNoActiveSave
	}

	autosaves := filterAutosaves(headers)
	if len(autosaves) == 0 {
		autosaves = headers
	}

	best := autosaves[0]
	bestTime, err := parseSaveDateTime(best.SaveDateTime)
	if err != nil {
		return SaveHeader{}, err
	}

	for _, h := range autosaves[1:] {
		t, err := parseSaveDateTime(h.SaveDateTime)
		if err != nil {
			continue
		}
		if t.After(bestTime) {
			best = h
			bestTime = t
		}
	}
	if strings.TrimSpace(best.SaveName) == "" {
		return SaveHeader{}, ErrNoActiveSave
	}
	return best, nil
}

func filterAutosaves(headers []SaveHeader) []SaveHeader {
	out := make([]SaveHeader, 0, len(headers))
	for _, h := range headers {
		if strings.Contains(h.SaveName, "_autosave_") {
			out = append(out, h)
		}
	}
	return out
}

func parseSaveDateTime(raw string) (time.Time, error) {
	t, err := time.Parse(saveDateTimeLayout, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}
