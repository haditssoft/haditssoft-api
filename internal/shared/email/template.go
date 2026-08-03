package email

import (
	"github.com/matcornic/hermes/v2"
)

func GenerateVerificationCodeTemplate(userName string, code string) (string, string) {
	emailBody := `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
  body { margin: 0; padding: 0; background-color: #f4f4f7; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; }
  .container { max-width: 560px; margin: 0 auto; background-color: #ffffff; }
  .header { background-color: #3f51b5; padding: 24px 32px; text-align: center; }
  .header h1 { color: #ffffff; margin: 0; font-size: 20px; font-weight: 600; }
  .content { padding: 32px; }
  .greeting { font-size: 16px; color: #333333; margin: 0 0 16px; }
  .message { font-size: 14px; color: #555555; line-height: 1.6; margin: 0 0 24px; }
  .code-box { background-color: #f0f0f5; border: 2px dashed #3f51b5; border-radius: 8px; padding: 20px 24px; text-align: center; margin: 0 0 24px; }
  .code-label { font-size: 12px; color: #888888; text-transform: uppercase; letter-spacing: 1px; margin: 0 0 8px; }
  .code { font-size: 32px; font-weight: 700; font-family: 'Courier New', Courier, monospace; color: #3f51b5; letter-spacing: 8px; margin: 0; user-select: all; cursor: text; }
  .footer { padding: 16px 32px 24px; border-top: 1px solid #e8e8eb; }
  .footer p { font-size: 12px; color: #999999; margin: 0 0 4px; line-height: 1.5; }
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1>HaditsSoft</h1>
  </div>
  <div class="content">
    <p class="greeting">Hi ` + userName + `,</p>
    <p class="message">Welcome to HaditsSoft! Please use the verification code below to verify your email address.</p>
    <div class="code-box">
      <p class="code-label">Your Verification Code</p>
      <p class="code">` + code + `</p>
    </div>
    <p class="message">This code expires in <strong>15 minutes</strong>. If you did not create an account, you can safely ignore this email.</p>
  </div>
  <div class="footer">
    <p>&copy; HaditsSoft. All rights reserved.</p>
  </div>
</div>
</body>
</html>`

	emailText := "Hi " + userName + ",\n\nYour verification code is: " + code + "\n\nThis code expires in 15 minutes."

	return emailBody, emailText
}

func GenerateTemplate() (string, string) {
	h := hermes.Hermes{
		Product: hermes.Product{
			Name: "Hermes",
			Link: "https://example-hermes.com/",
			Logo: "http://www.duchess-france.org/wp-content/uploads/2016/01/gopher.png",
		},
	}

	email := hermes.Email{
		Body: hermes.Body{
			Name: "Jon Snow",
			Intros: []string{
				"Welcome to Hermes! We're very excited to have you on board.",
			},
			Actions: []hermes.Action{
				{
					Instructions: "To get started with Hermes, please click here:",
					Button: hermes.Button{
						Color: "#22BC66",
						Text:  "Confirm your account",
						Link:  "https://hermes-example.com/confirm?token=d9729feb74992cc3482b350163a1a010",
					},
				},
			},
			Outros: []string{
				"Need help, or have questions? Just reply to this email, we'd love to help.",
			},
		},
	}

	emailBody, err := h.GenerateHTML(email)
	if err != nil {
		panic(err)
	}

	emailText, err := h.GeneratePlainText(email)
	if err != nil {
		panic(err)
	}

	return emailBody, emailText
}
