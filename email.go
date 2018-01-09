package main

import "net/smtp"

func sendemail(body string, subj string) {
	from := config.Emailfrom
	pass := config.Emailpwd
	to := config.Emailto
	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: " + subj + "\n\n" +
		body

	err := smtp.SendMail("smtp.gmail.com:587",
		smtp.PlainAuth("", from, pass, "smtp.gmail.com"),
		from, []string{to}, []byte(msg))

	if err != nil {
		Error.Printf("smtp error: %s", err.Error())
		return
	}
}
