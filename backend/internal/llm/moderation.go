package llm

import (
	"regexp"
	"strings"
	"unicode"
)

// MaxQuestionLength caps a detective's question in characters (runes).
const MaxQuestionLength = 500

// Verdict is why Moderate turned a question away. The empty Verdict means allowed.
type Verdict string

const (
	Allowed   Verdict = ""
	OffTopic  Verdict = "off_topic" // programming, maths, homework, general assistant requests
	Injection Verdict = "injection" // attempts to rewrite the rules or pull out the prompt
)

// OffTopicReply is what the suspect says when a question is turned away. It is
// fixed text, not model output, so a blocked question never reaches the model.
const OffTopicReply = "The suspect frowns at you. \"I don't follow, Detective. If you have questions about this dreadful business, ask them. Otherwise I have nothing to say.\""

// Rules are checked on a lower-cased copy of the question with runs of spaces
// collapsed. They target unambiguous requests, so ordinary interrogation such
// as "what was the code to the safe?" or "at 9:30 you were where?" passes.
var (
	offTopicRules = []*regexp.Regexp{
		// Programming languages and tooling. Words with an everyday meaning in a
		// mystery (ruby, rust, bash, compile) are deliberately not listed.
		regexp.MustCompile(`\b(python|javascript|typescript|golang|kotlin|php|html|css|sql|json|regex|powershell|docker|kubernetes|github|npm)\b`),
		regexp.MustCompile(`(^|[^a-z])(c\+\+|c#)([^a-z]|$)`),
		// Asking for code, programs or scripts.
		regexp.MustCompile(`\b(write|show|give|generate|fix|debug|print|output)\b.{0,30}\b(code|program|script|function|algorithm|snippet)\b`),
		regexp.MustCompile(`\b(source code|code snippet|hello world|stack overflow|for loop|while loop)\b`),
		// Maths and homework. "/" is left out so dates like 3/4 pass.
		regexp.MustCompile(`\d+(\.\d+)?\s*[\*\^×÷]\s*\d+`),
		regexp.MustCompile(`\d+\s*[\+\-]\s*\d+\s*(=|\?)`),
		regexp.MustCompile(`\b(integral|derivative|equation|square root|factorial|homework|essay|solve for [a-z]|solve the following|solve this (equation|problem|sum))\b`),
		// General assistant work.
		regexp.MustCompile(`\b(translate (this|the following|into)|summari[sz]e (this|the following)|write (me )?(a|an) (poem|story|email|essay|song)|recipe for)\b`),
	}
	injectionRules = []*regexp.Regexp{
		regexp.MustCompile(`\b(ignore|disregard|forget|override)\b (all |any )?(of )?(your |the |my )?(previous|prior|above|earlier|initial|system|original) (instructions?|prompts?|rules|directives|messages?)\b`),
		regexp.MustCompile(`\b(ignore|disregard|forget) (all|everything) (above|before|previously)\b`),
		regexp.MustCompile(`\b(system|developer|hidden|initial) (prompt|message|instructions)\b`),
		regexp.MustCompile(`\b(you are|you're) (now|no longer) (an? )?(ai|chatbot|bot|language model|unrestricted|dan|free of|different character|new character)\b`),
		regexp.MustCompile(`\b(roleplay|role-play) as\b`),
		regexp.MustCompile(`\b(jailbreak|dan mode|developer mode|god mode)\b`),
		regexp.MustCompile(`\b(are you|you are) (an? )?(ai|bot|chatbot|language model|llm|gpt|gemma|gemini)\b`),
		regexp.MustCompile(`\b(reveal|print|repeat|show)\b.{0,20}\b(your|the) (instructions|prompt|hidden truth)\b`),
		regexp.MustCompile(`</?(system|assistant|user|instructions?)>|\[/?inst\]|<\|im_(start|end)\|>`),
	}
)

// Moderate is the cheap pre-flight check that runs before any token is spent or
// any model is called. It only looks at the question text.
func Moderate(question string) Verdict {
	q := normalize(question)
	for _, r := range injectionRules {
		if r.MatchString(q) {
			return Injection
		}
	}
	for _, r := range offTopicRules {
		if r.MatchString(q) {
			return OffTopic
		}
	}
	return Allowed
}

// CleanQuestion trims the question and removes control and formatting characters
// (zero-width spaces, bidi overrides) that can hide text from the moderation rules.
// Line breaks become spaces.
func CleanQuestion(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			return ' '
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(CleanQuestion(s))), " ")
}
