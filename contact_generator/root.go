package contact_generator

import (
	"MailContactUtilty/helper"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

type ContactGenerator struct {
	ctx    context.Context
	client *genai.Client
	model  *genai.GenerativeModel
}

func NewContactGenerator(ctx context.Context) *ContactGenerator {
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}
	model := client.GenerativeModel("gemini-2.0-flash-lite")
	model.ResponseMIMEType = "application/json"
	model.ResponseSchema = &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"Name":    {Type: genai.TypeString},
			"Surname": {Type: genai.TypeString},
			"Email":   {Type: genai.TypeString},
			"Phone":   {Type: genai.TypeString},
		},
	}
	return &ContactGenerator{
		ctx:    ctx,
		model:  model,
		client: client,
	}
}

func (c *ContactGenerator) Generate(mail string) helper.Contact {
	resp, err := c.model.GenerateContent(c.ctx, genai.Text("Extract the sender data, utilizing the data from the top of the mail, aswell as the footer, from this mail: \n"+mail+"\nBe very sure of the data you extract, if data is missing, do not make it up, but return an empty string instead, if the email or phone is different between the top and the footer, return the email or phone from the footer, be sure to include the data if the mail contains it"))
	if err != nil {
		log.Fatal(err)
	}
	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			var contact helper.Contact
			err := json.Unmarshal([]byte(fmt.Sprint(part)), &contact)
			if err == nil {
				return contact
			}
		}
	}
	return helper.Contact{}
}

func (c *ContactGenerator) Close() error {
	c.client.Close()
	return nil

}
