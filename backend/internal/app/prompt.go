package app

import (
	"fmt"
	"strings"

	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// GenerationInstructions is the provider-neutral system/task prompt.
// Adapters attach the document (as a PDF part or as extracted text).
func GenerationInstructions(spec domain.QuestionSpec, feedback []string) string {
	types := make([]string, len(spec.TypeMix))
	for i, t := range spec.TypeMix {
		types[i] = string(t)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `You write practice questions for a student studying the attached document.

Produce EXACTLY %d questions as JSON matching the provided schema.
Use a mix of these types: %s.

Rules per type:
- mcq: exactly 4 distinct options, exactly 1 correct index.
- true_false: options exactly ["True", "False"], exactly 1 correct index.
- mrq ("select all that apply"): 3 to 5 distinct options, at least 2 correct and at least 1 incorrect.

Quality rules:
- Every question must be answerable from the document alone; do not use outside knowledge.
- Cover different sections of the document; avoid near-duplicate questions.
- Distractors must be plausible but clearly wrong according to the document.
- Do not use "all of the above" or "none of the above".
- correctIndices are zero-based indices into options.
- explanation: 1-3 sentences explaining why, referencing the document.
- sourceHint: the section heading or page the question comes from.
`, spec.Count, strings.Join(types, ", "))
	if len(feedback) > 0 {
		b.WriteString("\nYour previous answer was rejected for these reasons; fix them:\n")
		for _, f := range feedback {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	return b.String()
}
