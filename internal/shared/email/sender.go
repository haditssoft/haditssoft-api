package email

import (
	"os"
	"strconv"

	gomail "gopkg.in/gomail.v2"
)

func Sender(to []string, subject string) error {
	html, _ := GenerateTemplate()

	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("MAIL_FROM_NAME")+" <"+os.Getenv("MAIL_FROM_ADDRESS")+">")
	for _, s := range to {
		m.SetHeader("To", s)
	}
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", html)

	port, err := strconv.Atoi(os.Getenv("MAIL_PORT"))
	if err != nil {
		return err
	}

	d := gomail.NewDialer(
		os.Getenv("MAIL_HOST"),
		port,
		os.Getenv("MAIL_USERNAME"),
		os.Getenv("MAIL_PASSWORD"),
	)

	if err := d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}

func SendVerificationCode(to []string, userName string, code string) error {
	html, _ := GenerateVerificationCodeTemplate(userName, code)

	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("MAIL_FROM_NAME")+" <"+os.Getenv("MAIL_FROM_ADDRESS")+">")
	for _, s := range to {
		m.SetHeader("To", s)
	}
	m.SetHeader("Subject", "Your Verification Code")
	m.SetBody("text/html", html)

	port, err := strconv.Atoi(os.Getenv("MAIL_PORT"))
	if err != nil {
		return err
	}

	d := gomail.NewDialer(
		os.Getenv("MAIL_HOST"),
		port,
		os.Getenv("MAIL_USERNAME"),
		os.Getenv("MAIL_PASSWORD"),
	)

	if err := d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}
