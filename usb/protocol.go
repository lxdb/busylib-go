package usb

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var ansiSequence = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func buildCommand(command string, args ...string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" || strings.ContainsAny(command, " \t\r\n\x00") {
		return command, ErrInvalidCommand
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, command)
	for _, argument := range args {
		if strings.ContainsAny(argument, "\r\n\x00") {
			return strings.Join(append(parts, argument), " "), ErrInvalidCommand
		}
		parts = append(parts, argument)
	}
	return strings.Join(parts, " "), nil
}

func readUntilPrompt(reader io.Reader, maximum int) ([]byte, error) {
	result := make([]byte, 0, 1024)
	buffer := make([]byte, 4096)
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			result = append(result, buffer[:count]...)
			if len(result) > maximum {
				return result, ErrResponseTooLarge
			}
			if containsPrompt(result) {
				return result, nil
			}
		}
		if err != nil {
			return result, fmt.Errorf("%w: %w", ErrPromptNotFound, err)
		}
	}
}

func containsPrompt(data []byte) bool {
	return bytes.HasSuffix(stripIAC(data), []byte(Prompt))
}

func cleanOutput(raw []byte, command string) string {
	clean := stripIAC(raw)
	clean = ansiSequence.ReplaceAll(clean, nil)
	if index := bytes.LastIndex(clean, []byte(Prompt)); index >= 0 {
		clean = clean[:index]
	}
	text := strings.ReplaceAll(string(clean), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == command {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func stripIAC(data []byte) []byte {
	result := make([]byte, 0, len(data))
	for index := 0; index < len(data); {
		if data[index] != 255 {
			result = append(result, data[index])
			index++
			continue
		}
		if index+1 >= len(data) {
			break
		}
		command := data[index+1]
		switch command {
		case 255:
			result = append(result, 255)
			index += 2
		case 251, 252, 253, 254:
			if index+2 >= len(data) {
				return result
			}
			index += 3
		case 250:
			index += 2
			for index+1 < len(data) && (data[index] != 255 || data[index+1] != 240) {
				index++
			}
			if index+1 < len(data) {
				index += 2
			}
		default:
			index += 2
		}
	}
	return result
}

func trailingBytes(data []byte, count int) []byte {
	if count <= 0 {
		return nil
	}
	if len(data) <= count {
		return append([]byte(nil), data...)
	}
	return append([]byte(nil), data[len(data)-count:]...)
}
