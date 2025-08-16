package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Action string

const (
	ActionApply      Action = "apply"
	ActionRegenerate Action = "regenerate"
	ActionCancel     Action = "cancel"
)

type InteractivePrompt struct {
	reader *bufio.Reader
}

func New() *InteractivePrompt {
	return &InteractivePrompt{
		reader: bufio.NewReader(os.Stdin),
	}
}

func (p *InteractivePrompt) AskForAction(message string) (Action, error) {
	fmt.Println()
	if message != "" {
		fmt.Println(message)
	}
	fmt.Print("\n[A]pply / [R]egenerate / [C]ancel? ")
	
	input, err := p.reader.ReadString('\n')
	if err != nil {
		return ActionCancel, err
	}
	
	input = strings.TrimSpace(strings.ToLower(input))
	
	switch input {
	case "a", "apply":
		return ActionApply, nil
	case "r", "regenerate":
		return ActionRegenerate, nil
	case "c", "cancel", "":
		return ActionCancel, nil
	default:
		return p.AskForAction("Invalid choice. Please enter A, R, or C.")
	}
}

func (p *InteractivePrompt) Confirm(message string) (bool, error) {
	fmt.Print(message + " [y/N]: ")
	
	input, err := p.reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

func (p *InteractivePrompt) GetInput(prompt string) (string, error) {
	fmt.Print(prompt)
	
	input, err := p.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(input), nil
}