// Command genfixture writes a small text-only sample PDF for genbench.
package main

import (
	"fmt"
	"os"

	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/pdftest"
)

var pages = []string{
	"Cell Biology 101. Cells are the basic structural and functional units of life.",
	"The nucleus contains DNA and controls gene expression. The nucleolus makes ribosomes.",
	"Mitochondria produce ATP through cellular respiration and have their own DNA.",
	"Ribosomes synthesise proteins. Rough ER is studded with ribosomes; smooth ER makes lipids.",
	"The Golgi apparatus modifies, sorts and packages proteins for secretion.",
	"Plant cells have a cellulose cell wall, chloroplasts for photosynthesis and a large central vacuole.",
	"Animal cells lack a cell wall and chloroplasts but have centrioles and lysosomes.",
	"The plasma membrane is a phospholipid bilayer that is selectively permeable.",
	"Diffusion moves molecules from high to low concentration; active transport uses ATP.",
	"Prokaryotes such as bacteria lack a nucleus and membrane-bound organelles.",
}

func main() {
	out := "testdata/pdfs/sample-cells.pdf"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.WriteFile(out, pdftest.Minimal(pages...), 0o600); err != nil { //nolint:gosec // developer-chosen output path
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", out)
}
