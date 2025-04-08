package contact_generator

import (
	"MailContactUtilty/helper"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type ContactGenerator struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

func NewContactGenerator(ctx context.Context, apiKey string) (*ContactGenerator, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	model := client.GenerativeModel("gemini-2.0-flash-lite")
	model.ResponseMIMEType = "application/json"
	model.ResponseSchema = &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"Name":         {Type: genai.TypeString},
			"Surname":      {Type: genai.TypeString},
			"Email":        {Type: genai.TypeString},
			"Phone":        {Type: genai.TypeString},
			"Organization": {Type: genai.TypeString},
		},
	}
	return &ContactGenerator{
		model:  model,
		client: client,
	}, nil
}

func (c *ContactGenerator) Generate(ctx context.Context, mail string) (*helper.Contact, error) {
	resp, err := c.model.GenerateContent(ctx, genai.Text("Extract the sender data, utilizing the data from the top of the mail, aswell as the footer, from this mail: \n"+mail+"\nBe very sure of the data you extract, if data is missing, do not make it up, but return an empty string instead, if the email or phone is different between the top and the footer, return the email or phone from the footer, be sure to include the data if the mail contains it"))
	if err != nil {
		return nil, err
	}
	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			var contact helper.Contact
			err := json.Unmarshal([]byte(fmt.Sprint(part)), &contact)
			if err == nil {
				return &contact, nil
			}
		}
	}
	return nil, fmt.Errorf("no valid response found")
}

func (c *ContactGenerator) Close() error {
	c.client.Close()
	return nil
}
