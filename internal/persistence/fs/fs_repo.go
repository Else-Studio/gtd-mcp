package fs

import (
	"bytes"
	"os"
	"strings"
)

func atomicWrite(path string, data []byte) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func parseMarkdown(content []byte) (frontmatter []byte, title string, desc string, err error) {
	str := string(content)

	if strings.HasPrefix(str, "---\n") {
		str = str[4:]
		idx := strings.Index(str, "\n---\n")
		if idx != -1 {
			frontmatter = []byte(str[:idx])
			body := str[idx+5:]

			title, desc = parseBody(body)
			return frontmatter, title, desc, nil
		}
	}

	title, desc = parseBody(str)
	return nil, title, desc, nil
}

func parseBody(body string) (title string, desc string) {
	body = strings.TrimLeft(body, "\n")
	lines := strings.Split(body, "\n")
	if len(lines) > 0 {
		title = lines[0]
		if len(lines) > 1 {
			desc = strings.Join(lines[1:], "\n")
			desc = strings.TrimRight(desc, "\n")
		}
	}
	return title, desc
}

func formatMarkdown(frontmatter []byte, title string, desc string) []byte {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(frontmatter)
	if !bytes.HasSuffix(frontmatter, []byte("\n")) {
		buf.WriteString("\n")
	}
	buf.WriteString("---\n")

	buf.WriteString(title)
	buf.WriteString("\n")

	if desc != "" {
		buf.WriteString(desc)
		if !strings.HasSuffix(desc, "\n") {
			buf.WriteString("\n")
		}
	}
	return buf.Bytes()
}
