package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"os"
	"strings"
	"text/template"
	"time"
)

var bodyTmpl = template.Must(template.New("message").Parse(`From: KFC2K <{{.From}}>
To: {{.To}}
Date: {{.Date}}
Subject: [{{ .Status }}] {{.Alert.Labels.alertname}}: {{.Alert.Summary}}
{{.Alert.Description}}
Grouping labels:
{{range $label, $value := .Alert.Labels}}
  {{$label}} = "{{$value}}"{{end}}
Payload labels:
{{range $label, $value := .Alert.Payload}}
  {{$label}} = "{{$value}}"{{end}}`))

type notificationOp int

const (
	contentTypeJSON = "application/json"

	notificationOpTrigger notificationOp = iota
	notificationOpResolve
)

var (
	smtpSmartHost = flag.String("notification.smtp.smarthost", "", "Address of the smarthost to send all email notifications to.")
	smtpSender    = flag.String("notification.smtp.sender", "kfc@example.org", "Sender email address to use in email notifications.")
)

func writeEmailBody(w io.Writer, from, to, status string, a *Alert) error {
	return writeEmailBodyWithTime(w, from, to, status, a, time.Now())
}

func writeEmailBodyWithTime(w io.Writer, from, to, status string, a *Alert, moment time.Time) error {
	err := bodyTmpl.Execute(w, struct {
		From   string
		To     string
		Date   string
		Alert  *Alert
		Status string
	}{
		From:   from,
		To:     to,
		Date:   moment.Format("Mon, 2 Jan 2006 15:04:05 -0700"),
		Alert:  a,
		Status: status,
	})
	if err != nil {
		return err
	}
	return nil
}

func getSMTPAuth(hasAuth bool, mechs string) (smtp.Auth, *tls.Config, error) {
	if !hasAuth {
		return nil, nil, nil
	}

	username := os.Getenv("SMTP_AUTH_USERNAME")

	for _, mech := range strings.Split(mechs, " ") {
		switch mech {
		case "CRAM-MD5":
			secret := os.Getenv("SMTP_AUTH_SECRET")
			if secret == "" {
				continue
			}
			return smtp.CRAMMD5Auth(username, secret), nil, nil
		case "PLAIN":
			password := os.Getenv("SMTP_AUTH_PASSWORD")
			if password == "" {
				continue
			}
			identity := os.Getenv("SMTP_AUTH_IDENTITY")

			// We need to know the hostname for both auth and TLS.
			host, _, err := net.SplitHostPort(*smtpSmartHost)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid address: %s", err)
			}

			auth := smtp.PlainAuth(identity, username, password, host)
			cfg := &tls.Config{ServerName: host}
			return auth, cfg, nil
		}
	}
	return nil, nil, nil
}

func sendEmailNotification(to string, op notificationOp, a *Alert) error {
	status := ""
	switch op {
	case notificationOpTrigger:
		status = "ALERT"
	case notificationOpResolve:
		status = "RESOLVED"
	}
	// Connect to the SMTP smarthost.
	c, err := smtp.Dial(*smtpSmartHost)
	if err != nil {
		return err
	}
	defer c.Quit()

	// Authenticate if we and the server are both configured for it.
	auth, tlsConfig, err := getSMTPAuth(c.Extension("AUTH"))
	if err != nil {
		return err
	}

	if tlsConfig != nil {
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls failed: %s", err)
		}
	}

	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("%T failed: %s", auth, err)
		}
	}

	// Set the sender and recipient.
	c.Mail(*smtpSender)
	c.Rcpt(to)

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		return err
	}
	defer wc.Close()

	return writeEmailBody(wc, *smtpSender, status, to, a)
}

func main() {
	flag.Parse()
	fmt.Printf("smart host: %s, smtp sender: %s\n", *smtpSmartHost, *smtpSender)

	alert := &Alert{
		Summary:     "Hello world",
		Description: "Hello Kevin I'm alive",
		Labels: AlertLabelSet{
			"hello": "world",
		},
		Payload: AlertPayload{
			"GenertorURL": "http://www.blah.com",
		},
	}

	err := sendEmailNotification(*smtpSender, notificationOpResolve, alert)
	if err != nil {
		fmt.Printf(" Error happened: %v\n", err)
	}
}
