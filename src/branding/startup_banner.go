package branding

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
)

// Tagline matches the README and logo artwork.
const Tagline = "Agentless backup. Immutable copies. One-call restoration."

// PrintStartupBanner logs brand + tagline once at process start (TTY gets color).
func PrintStartupBanner() {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Println("CosmosBDR — " + Tagline)
		return
	}

	const (
		reset   = "\033[0m"
		bold    = "\033[1m"
		dim     = "\033[2m"
		cosmos  = "\033[97m"   // logo: Cosmos (TTY)
		bdrBlue = "\033[1;94m" // logo: BDR (see ColorBDRDisplayHex for web/README)
	)

	fmt.Println()
	fmt.Printf("  %s%sCosmos%s%s%sBDR%s\n", bold, cosmos, reset, bold, bdrBlue, reset)
	fmt.Printf("  %s%s%s\n", dim, Tagline, reset)
	fmt.Println()
}