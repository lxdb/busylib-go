package docscheck

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func TestDocumentationInlineLinksAndFences(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range documentationFiles(t, root) {
		path := path
		t.Run(filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator))), func(t *testing.T) {
			content := readFile(t, path)
			checkFences(t, content)
			checkInlineLinks(t, root, path, content)
		})
	}
}

func TestValidateLocalLinksChecksSameDocumentFragments(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	content := "# Present\n\n[Missing](#absent)\n"
	writeFixture(t, root, "guide.md", content)
	issues := validateLocalLinks(root, source, content)
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1: %v", len(issues), issues)
	}
}

func TestValidateLocalLinksIgnoresFencedCode(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	content := "# Real\n\n```markdown\n# Fake\n[Ignored](missing.md)\n```\n\n[Real](#real)\n"
	writeFixture(t, root, "guide.md", content)
	if issues := validateLocalLinks(root, source, content); len(issues) != 0 {
		t.Fatalf("reported issues for fenced Markdown: %v", issues)
	}
}

func TestValidateLocalLinksSupportsBalancedParentheses(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	content := "[Draft](guide_(draft).md)\n"
	writeFixture(t, root, "guide.md", content)
	writeFixture(t, root, "guide_(draft).md", "# Draft\n")
	if issues := validateLocalLinks(root, source, content); len(issues) != 0 {
		t.Fatalf("reported issues for balanced link target: %v", issues)
	}
}

func checkFences(t *testing.T, content string) {
	t.Helper()
	_, fence := markdownOutsideFences(content)
	if fence != "" {
		t.Errorf("unclosed %s code fence", fence)
	}
}

func markdownOutsideFences(content string) (string, string) {
	var prose strings.Builder
	var fenceCharacter byte
	var fenceLength int
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		original := scanner.Text()
		character, length := fenceMarker(strings.TrimSpace(original))
		if fenceCharacter == 0 && length >= 3 {
			fenceCharacter = character
			fenceLength = length
			continue
		}
		if fenceCharacter != 0 {
			if character == fenceCharacter && length >= fenceLength {
				fenceCharacter = 0
				fenceLength = 0
			}
			continue
		}
		prose.WriteString(original)
		prose.WriteByte('\n')
	}
	if fenceCharacter == 0 {
		return prose.String(), ""
	}
	return prose.String(), strings.Repeat(string(fenceCharacter), fenceLength)
}

func fenceMarker(line string) (byte, int) {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return 0, 0
	}
	character := line[0]
	length := 0
	for length < len(line) && line[length] == character {
		length++
	}
	return character, length
}

func checkInlineLinks(t *testing.T, root, source, content string) {
	t.Helper()
	for _, issue := range validateLocalLinks(root, source, content) {
		t.Error(issue)
	}
}

func validateLocalLinks(root, source, content string) []string {
	var issues []string
	prose, _ := markdownOutsideFences(content)
	for _, linkTarget := range inlineLinkTargets(prose) {
		target := strings.TrimSpace(linkTarget)
		if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") {
			target = strings.TrimSuffix(strings.TrimPrefix(target, "<"), ">")
		}
		if fields := strings.Fields(target); len(fields) > 0 {
			target = fields[0]
		}
		if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}

		pathPart, fragment, _ := strings.Cut(target, "#")
		decodedPath, err := url.PathUnescape(pathPart)
		if err != nil {
			issues = append(issues, fmt.Sprintf("invalid escaped link %q: %v", target, err))
			continue
		}
		resolved := source
		if decodedPath != "" {
			resolved = filepath.Clean(filepath.Join(filepath.Dir(source), filepath.FromSlash(decodedPath)))
		}
		if !strings.HasPrefix(resolved, root+string(filepath.Separator)) && resolved != root {
			issues = append(issues, fmt.Sprintf("link escapes repository: %q", target))
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil {
			issues = append(issues, fmt.Sprintf("link target does not exist: %q", target))
			continue
		}
		if fragment != "" && !info.IsDir() && filepath.Ext(resolved) == ".md" {
			decoded, err := url.PathUnescape(fragment)
			if err != nil {
				issues = append(issues, fmt.Sprintf("invalid escaped fragment %q: %v", fragment, err))
				continue
			}
			targetContent, err := os.ReadFile(resolved)
			if err != nil {
				issues = append(issues, fmt.Sprintf("read link target %q: %v", target, err))
				continue
			}
			if _, ok := markdownAnchors(string(targetContent))[decoded]; !ok {
				issues = append(issues, fmt.Sprintf("fragment #%s does not exist in %s", decoded, resolved))
			}
		}
	}
	return issues
}

func inlineLinkTargets(content string) []string {
	var targets []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		for offset := 0; offset < len(line); {
			relativeClose := strings.Index(line[offset:], "](")
			if relativeClose < 0 {
				break
			}
			closeBracket := offset + relativeClose
			if strings.LastIndex(line[:closeBracket], "[") < 0 {
				offset = closeBracket + 2
				continue
			}

			start := closeBracket + 2
			depth := 1
			end := start
			for end < len(line) {
				switch line[end] {
				case '\\':
					end++
				case '(':
					depth++
				case ')':
					depth--
					if depth == 0 {
						targets = append(targets, line[start:end])
						end++
						offset = end
						break
					}
				}
				if depth == 0 {
					break
				}
				end++
			}
			if depth != 0 {
				break
			}
		}
	}
	return targets
}

func markdownAnchors(content string) map[string]struct{} {
	anchors := make(map[string]struct{})
	counts := make(map[string]int)
	prose, _ := markdownOutsideFences(content)
	scanner := bufio.NewScanner(strings.NewReader(prose))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if heading == "" {
			continue
		}
		base := githubSlug(heading)
		slug := base
		if count := counts[base]; count > 0 {
			slug += "-" + strconv.Itoa(count)
		}
		counts[base]++
		anchors[slug] = struct{}{}
	}
	return anchors
}

func githubSlug(heading string) string {
	heading = strings.ToLower(strings.ReplaceAll(heading, "`", ""))
	var builder strings.Builder
	lastHyphen := false
	for _, character := range heading {
		switch {
		case unicode.IsLetter(character), unicode.IsNumber(character), character == '_':
			builder.WriteRune(character)
			lastHyphen = false
		case character == '-' || unicode.IsSpace(character):
			if !lastHyphen {
				builder.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}
