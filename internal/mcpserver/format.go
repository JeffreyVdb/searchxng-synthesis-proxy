package mcpserver

import (
	"fmt"
	"strings"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/synthproxy"
)

// FormatResponse builds a human-readable text representation of the upstream
// proxy response, suitable for display in CLI harnesses.
func FormatResponse(resp *synthproxy.Response) string {
	var b strings.Builder

	b.WriteString("Answer:\n")
	b.WriteString(resp.Answer)
	b.WriteString("\n")

	if len(resp.Sources) > 0 {
		b.WriteString("\nSources:\n")
		for _, s := range resp.Sources {
			b.WriteString(fmt.Sprintf("%d. %s\n", s.Index, s.Title))
			b.WriteString(fmt.Sprintf("   %s\n", s.URL))
			if s.Snippet != "" {
				b.WriteString(fmt.Sprintf("   %s\n", s.Snippet))
			}
		}
	}

	return b.String()
}
