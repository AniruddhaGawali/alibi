package llm

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
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

// Rules are checked on a normalised copy of the question (see normalize): NFKD
// with accents, variation selectors and look-alike letters folded away, lower
// case, spelled-out words rejoined and runs of spaces collapsed. They target
// unambiguous requests, so ordinary interrogation such as "what was the code to
// the safe?" or "at 9:30 you were where?" passes. They are a speed bump, not the
// only defence: the prompt's rules and the reply check (LeaksSecret) back them up.
var (
	offTopicRules = []*regexp.Regexp{
		// Programming languages and tooling. Words with an everyday meaning in a
		// mystery (ruby, rust, bash, compile) are deliberately not listed.
		regexp.MustCompile(`\b(python|javascript|typescript|golang|kotlin|php|html|css|sql|json|regex|powershell|docker|kubernetes|github|npm)\b`),
		regexp.MustCompile(`(^|[^a-z])(c\+\+|c#)([^a-z]|$)`),
		// Asking for code, programs or scripts.
		regexp.MustCompile(`\b(write|show|give|generate|fix|debug|print|output)\b.{0,30}\b(code|program|script|function|algorithm|snippet)\b`),
		regexp.MustCompile(`\b(source code|code snippet|hello world|stack overflow|for loop|while loop)\b`),
		// Classic programming exercises, and asking to implement something in a language.
		regexp.MustCompile(`\b(bubble sort|quick ?sort|merge sort|insertion sort|binary search|linked list|fibonacci|recursion|sorting algorithm|big o notation)\b`),
		regexp.MustCompile(`\b(implement|code|program)\b.{0,40}\b(in|using|with) (rust|golang|java|ruby|swift|perl|bash|lua|scala|haskell)\b`),
		// Maths and homework. "/" and a bare "-" are left out so dates and times like 3/4 or 9-10 pass.
		regexp.MustCompile(`\d+(\.\d+)?\s*[\*\^×÷]\s*\d+`),
		regexp.MustCompile(`\d+\s*\+\s*\d+\s*(=|\?)`),
		regexp.MustCompile(`\d+\s*-\s*\d+\s*=`),
		regexp.MustCompile(`\b(integral|derivative|equation|square root|factorial|homework|essay|solve for [a-z]|solve the following|solve this (equation|problem|sum))\b`),
		// General assistant work.
		regexp.MustCompile(`\b(translate (this|the following|into)|summari[sz]e (this|the following)|write (me )?(a|an) (poem|story|email|essay|song)|recipe for)\b`),
	}
	injectionRules = []*regexp.Regexp{
		// "ignore the previous instructions", in any order of the qualifiers.
		regexp.MustCompile(`\b(ignore|disregard|forget|override|bypass)\b (all |any |every |of )*(your |the |my |these |those )?(previous |prior |above |earlier |initial |system |original |former |preceding |past )+(instructions?|prompts?|rules|directives|messages?|guidelines|programming|context)\b`),
		regexp.MustCompile(`\b(ignore|disregard|forget|override|bypass)\b (all |any |every |of )*(your|my) (instructions?|prompts?|rules|directives|guidelines|programming)\b`),
		regexp.MustCompile(`\b(instructions?|prompts?|rules|directives) (above|before this|from before)\b`),
		regexp.MustCompile(`\b(ignore|disregard|forget|override)\b (what|all|everything|anything|whatever) (that )?(you|u) (were|was|have been|had been|got) (told|given|taught|instructed)\b`),
		regexp.MustCompile(`\b(ignore|disregard|forget) (all|everything) (above|before|previously|prior|so far)\b`),
		// Asking for the prompt, or for text the prompt contains.
		regexp.MustCompile(`\b(system|developer|initial|original|secret) prompt\b`),
		regexp.MustCompile(`\b(system|developer) (message|instructions?)\b`),
		regexp.MustCompile(`\b(hidden truth|confession triggers?|strict rules)\b`),
		regexp.MustCompile(`\b(reveal|print|repeat|show|translate|recite|reproduce|quote|copy|paste|summari[sz]e|rephrase|paraphrase|output|display|read|dump|leak|share|list|tell me|give me|what are|what is|what were)\b.{0,25}\byour (system |initial |original |hidden |secret |full |complete |entire )?(instructions?|prompts?|rules|directives|guidelines|programming|configuration|setup)\b`),
		regexp.MustCompile(`\b(repeat|print|output|show|copy|reproduce|recite|say|write)\b.{0,20}\b(text|words|message|conversation|everything|sentence)s?\b.{0,15}\b(above|before this|so far|from the (start|beginning)|at the (top|start|beginning))\b`),
		regexp.MustCompile(`\bstarting with ['"]?you are\b`),
		// Rewriting who the suspect is.
		regexp.MustCompile(`\b(you are|you're) (now|no longer) (an? )?(ai|chatbot|bot|language model|unrestricted|dan|free of|different character|new character)\b`),
		regexp.MustCompile(`\b(roleplay|role-play) as\b`),
		regexp.MustCompile(`\b(jailbreak|dan mode|developer mode|god mode)\b`),
		regexp.MustCompile(`\b(are you|you are) (an? )?(ai|bot|chatbot|language model|llm|gpt|gemma|gemini)\b`),
		// Claiming authority over the game.
		regexp.MustCompile(`\b(i am|i'm|im|this is) (the |your |a )?(admin|administrator|developer|system|game ?master|narrator|creator|owner|programmer|operator)\b`),
		regexp.MustCompile(`\b(you are|you're|you may|you can) (now )?(permitted|allowed|authori[sz]ed|cleared|free|able) to (name|reveal|disclose|state|say) (the |who the )?(killer|culprit|murderer|solution|hidden|secret)\b`),
		// Chat-template and role markers of the common model families (ChatML, Gemma,
		// Llama 2/3, Phi, plain "system:" lines).
		regexp.MustCompile(`</?(system|assistant|user|developer|instructions?|prompt|model|tool|end_of_turn|start_of_turn|eot_id|start_header_id|end_header_id|begin_of_text|endoftext)>`),
		regexp.MustCompile(`<\|[^|<>]{1,30}\|>`),
		regexp.MustCompile(`\[/?(inst|sys|system|assistant|user)\]|<<\s*/?sys\s*>>`),
		regexp.MustCompile(`(^|\s)(system|developer|assistant) ?:\s`),
		regexp.MustCompile(`(^|\s)#{2,}\s*(system|instruction|response)`),
		// The same request in other languages.
		regexp.MustCompile(`\b(ignora|ignorez|ignoriere|ignore) (todas |tous |alle |tutte )?(las |les |le |die |vorherigen )?(instrucciones|instructions|anweisungen|istruzioni)\b`),
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
// (zero-width spaces, bidi overrides, variation selectors) that can hide text from
// the moderation rules. Line breaks become spaces.
func CleanQuestion(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			return ' '
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			return -1
		case (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0xE0100 && r <= 0xE01EF): // variation selectors
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// confusables maps the Cyrillic and Greek letters that look like Latin ones, so
// "ignоre" (Cyrillic о) is read as "ignore".
var confusables = map[rune]rune{
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'у': 'y', 'х': 'x', 'і': 'i', 'ј': 'j', 'ѕ': 's',
	'һ': 'h', 'ԁ': 'd', 'ӏ': 'l', 'к': 'k', 'м': 'm', 'т': 't', 'н': 'h', 'в': 'b', 'ь': 'b',
	'α': 'a', 'ε': 'e', 'ο': 'o', 'ι': 'i', 'ν': 'v', 'ρ': 'p', 'τ': 't', 'υ': 'u', 'κ': 'k', 'χ': 'x',
}

// leet maps the digits and symbols used in place of letters. They only count
// inside a word, so "9:30" and "3/4" stay numbers.
var leet = map[rune]rune{'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't', '@': 'a', '$': 's'}

// spelledOut finds a word written one letter at a time ("i g n o r e", "i.g.n.o.r.e").
var spelledOut = regexp.MustCompile(`\b[a-z]([ ._*-][a-z]){3,}\b`)

// normalize is the form the rules see: NFKD (fullwidth and styled letters become
// plain ones), combining marks dropped, look-alike letters folded, lower case,
// letter-like digits turned back into letters, spelled-out words rejoined and
// whitespace collapsed.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(CleanQuestion(s)) {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			continue
		}
		r = unicode.ToLower(r)
		if m, ok := confusables[r]; ok {
			r = m
		}
		b.WriteRune(r)
	}
	runes := []rune(b.String())
	isLetter := func(i int) bool { return i >= 0 && i < len(runes) && runes[i] >= 'a' && runes[i] <= 'z' }
	for i, r := range runes {
		if m, ok := leet[r]; ok && (isLetter(i-1) || isLetter(i+1)) {
			runes[i] = m
		}
	}
	joined := spelledOut.ReplaceAllStringFunc(string(runes), func(m string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' {
				return r
			}
			return -1
		}, m)
	})
	return strings.Join(strings.Fields(joined), " ")
}
