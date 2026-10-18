package llm

import "strings"

// Unlocked reports whether the suspect's secrets may go into the prompt: the detective
// has touched on what they hide in this question or in an earlier one of the same
// interrogation (questions is the recent history plus the new question). A suspect with
// no TriggerKeywords is always unlocked.
//
// A keyword is a group of terms joined by "+"; every term of the group must be the start
// of a word in one question ("last+study" matches "Who was last in the study?", "card"
// matches "cards" and "card-table"). Any matching group unlocks. This is the cheap
// server-side check: the model never sees the secrets before it, so a player cannot
// talk or inject them out of a suspect who has not been asked the right thing.
func Unlocked(p Persona, questions []string) bool {
	if len(p.TriggerKeywords) == 0 {
		return true
	}
	for _, q := range questions {
		w := words(q)
		for _, group := range p.TriggerKeywords {
			if groupMatches(group, w) {
				return true
			}
		}
	}
	return false
}

func groupMatches(group string, questionWords []string) bool {
	terms := strings.Split(strings.ToLower(group), "+")
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" {
			return false
		}
		found := false
		for _, w := range questionWords {
			if strings.HasPrefix(w, t) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
