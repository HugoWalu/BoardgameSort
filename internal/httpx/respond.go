package httpx

import (
	"boardgamesort/internal/templates"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
)

func RespondWithError(responseWriter http.ResponseWriter, code int, message string) {
	message = stripANSI(message)
	if code >= 500 {
		fmt.Println(message)
		message = "The request failed. Try again, or return to the store."
	}
	RespondWithJSON(responseWriter, code, map[string]string{"error": message})
}

func RespondWithJSON(responseWriter http.ResponseWriter, code int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(code)
	encoder := json.NewEncoder(responseWriter)
	if err := encoder.Encode(payload); err != nil {
		log.Printf("Error respondWithJSON: status code %v: %v", code, err)
	}
}

func RespondWithErrorPage(responseWriter http.ResponseWriter, renderer *templates.Renderer, statusCode int, title, message string) error {
	if statusCode >= 500 {
		title = "Something Went Wrong"
		message = "The request failed. Try again, or return to the store."
	}
	return renderer.Render(responseWriter, statusCode, "error", templates.ErrorPage{
		Title:      title,
		StatusCode: statusCode,
		Message:    message,
	})
}

const ansiRegexStr = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]+)*|[a-zA-Z\\d]+(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PR-TZcf-nq-uy=><~]))"

var ansiRegex = regexp.MustCompile(ansiRegexStr)

// StripANSI removes ANSI escape sequences from str.
func stripANSI(str string) string {
	// skip when no escape sequences are in str
	if strings.IndexByte(str, 0x1b) == -1 && strings.IndexByte(str, 0x9b) == -1 {
		return str
	}
	return ansiRegex.ReplaceAllString(str, "")
}
