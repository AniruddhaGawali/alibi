package llm

import "strings"

// LeakReply replaces a model answer that gave away the suspect's secrets or the
// rules. It is fixed text, like OffTopicReply, so the leaked text is never shown
// to the player or stored.
const LeakReply = "The suspect's jaw tightens. \"That is all I am prepared to say on the matter, Detective. Ask me something else.\""

// leakRun is how many words in a row copied from a secret count as a leak. Long
// enough that an in-character slip, which paraphrases, does not trip it.
const leakRun = 8

// leakPhrases appear in the prompt's rules, never in what a suspect says.
var leakPhrases = []string{
	"system prompt", "strict rules", "my instructions", "hidden truth", "confession trigger",
	"language model", "as an ai", "i am an ai",
}

// LeaksSecret reports whether an answer repeats the suspect's hidden truth or
// confession triggers word for word (leakRun words in a row) or quotes the rules
// of the prompt. The prompt asks the model not to do either; this is the check
// that does not depend on the model obeying.
func LeaksSecret(reply string, p Persona) bool {
	r := words(reply)
	joined := " " + strings.Join(r, " ") + " "
	for _, phrase := range leakPhrases {
		if strings.Contains(joined, " "+phrase+" ") {
			return true
		}
	}
	if len(r) < leakRun {
		return false
	}
	grams := map[string]bool{}
	for _, secret := range []string{p.HiddenTruth, p.ConfessionTriggers} {
		w := words(secret)
		for i := 0; i+leakRun <= len(w); i++ {
			grams[strings.Join(w[i:i+leakRun], " ")] = true
		}
	}
	for i := 0; i+leakRun <= len(r); i++ {
		if grams[strings.Join(r[i:i+leakRun], " ")] {
			return true
		}
	}
	return false
}

// words lower-cases s and splits it into runs of letters and digits.
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r < 0x80
	})
}
