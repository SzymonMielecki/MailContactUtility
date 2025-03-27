package helper

import (
	"fmt"
	"github.com/google/generative-ai-go/genai"
)

type Contact struct {
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
}

func PrintResponse(resp *genai.GenerateContentResponse) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				fmt.Println(part)
			}
		}
	}
	fmt.Println("---")
}
