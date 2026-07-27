package larkspur

import (
	"fmt"
	"testing"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

type contentStrTest struct {
	content  any
	expected string
}

func TestChat(t *testing.T) {
	t.Run("Testing contentStr", testContentStr)
	t.Run("Testing messagesSize", testMessagesSize)
}

func testContentStr(t *testing.T) {
	fmt.Println(t.Name())

	tests := []contentStrTest{
		{content: "hello", expected: "hello"},
		{content: "", expected: ""},
		{content: 42, expected: "42"},
	}

	for _, test := range tests {
		got := contentStr(test.content)
		if got != test.expected {
			t.Fatalf("Expected `%s`, received `%s`", test.expected, got)
		}
	}
}

func testMessagesSize(t *testing.T) {
	fmt.Println(t.Name())

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: "abc"},
		{Role: anyllm.RoleUser, Content: "de"},
		{Role: anyllm.RoleTool, Content: ""},
	}

	if got := messagesSize(messages); got != 5 {
		t.Fatalf("Expected `5`, received `%d`", got)
	}

	if got := messagesSize(nil); got != 0 {
		t.Fatalf("Expected `0`, received `%d`", got)
	}
}
