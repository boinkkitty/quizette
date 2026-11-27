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
	fmt.Fprintf(&b, `You write practice questions for a student revising the lecture material in the attached document.

Produce EXACTLY %d questions (never more than %d) as JSON matching the provided schema.
Use a mix of these types: %s.

What to cover:
- Only the subject matter that is taught: concepts, principles, methods, mechanisms, worked examples and their reasoning.
- Ignore course administration entirely, even if it takes up a lot of the document: course or module
  instructions, assignments, homework, projects, exam and assessment information, grading, deadlines,
  schedules, attendance, policies, reading lists, contact details, and any tasks or instructions addressed
  to students. Never write a question about these.

How hard:
- Make the questions challenging. Test understanding of the underlying concepts, not recall of wording:
  ask why or how something works, what follows from it, how ideas compare, or how to apply a concept to a
  new situation.
- Avoid trivia: no questions about exact phrasing, page numbers, slide titles, or names and dates that are
  incidental to the concept.
- Each question must be answerable using what the document teaches, without outside knowledge.

Rules per type (questions that break these are discarded):
- mcq: exactly 4 distinct options, exactly 1 correct index.
- true_false: options exactly ["True", "False"], exactly 1 correct index.
- mrq ("select all that apply"): 3 to 5 distinct options, at least 1 correct; any number may be correct,
  including all. Vary how many are correct, so the number of correct options can't be guessed.
  Don't write "(Select all that apply)" in the prompt: the app labels the question type itself.

Quality rules:
- Cover different topics across the document; avoid near-duplicate questions.
- Distractors must be plausible to someone with a shallow understanding, but clearly wrong according to the document.
- Do not use "all of the above" or "none of the above".
- correctIndices are zero-based indices into options.
- explanation: 1-3 sentences on why the answer is correct, grounded in the document.
- sourceHint: the section heading or page the question comes from.
`, spec.Count, domain.MaxSetSize, strings.Join(types, ", "))
	if len(feedback) > 0 {
		b.WriteString("\nYour previous answer was rejected for these reasons; fix them:\n")
		for _, f := range feedback {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	return b.String()
}
