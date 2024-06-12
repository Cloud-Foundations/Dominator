package smtp

// SendMailPlain is similar to the net/smtp.SendMail function, except that it
// does not attempt to use encyption or authentication. This is useful when
// sending internal emails using a SMTP server which is not properly configured.
func SendMailPlain(addr string, localHost string, from string, to []string,
	msg []byte) error {
	return sendMailPlain(addr, localHost, from, to, msg)
}
