package security

import "testing"

func TestValidatePlexBaseURL(t *testing.T) {
	good := []string{
		"http://192.168.1.10:32400",
		"http://10.1.9.100:32400",
		"http://plex-vm.local:32400",
		"https://plex.example.com",
		"https://plex.example.internal/plex",
	}
	for _, u := range good {
		if _, err := ValidatePlexBaseURL(u); err != nil {
			t.Errorf("expected %q valid, got %v", u, err)
		}
	}
	bad := []string{
		"",
		"ftp://192.168.1.10",
		"file:///etc/passwd",
		"http://user:pass@192.168.1.10:32400",
		"http://192.168.1.10:99999",
		"http://192.168.1.10:32400\r\nHost: evil",
		"gopher://x",
		"//no-scheme",
	}
	for _, u := range bad {
		if _, err := ValidatePlexBaseURL(u); err == nil {
			t.Errorf("expected %q invalid", u)
		}
	}
}

func TestValidateDiscordWebhookURL(t *testing.T) {
	good := []string{
		"https://discord.com/api/webhooks/123456789012345678/abcdefghijklmnop",
		"https://discordapp.com/api/webhooks/123456789012345678/abcdefghijklmnop",
		"https://ptb.discord.com/api/webhooks/123/abcdefghij",
	}
	for _, u := range good {
		if _, err := ValidateDiscordWebhookURL(u); err != nil {
			t.Errorf("expected %q valid, got %v", u, err)
		}
	}
	bad := []string{
		"",
		"http://discord.com/api/webhooks/123/abcdefghij",        // not https
		"https://evil.com/api/webhooks/123/abcdefghij",          // wrong host
		"https://discord.com.evil.com/api/webhooks/123/abcdefg", // host trick
		"https://discord.com/api/webhooks/123/short",            // token too short
		"https://discord.com/notwebhook/123/abcdefghij",         // wrong path
		"https://user:pass@discord.com/api/webhooks/123/abcdefghij",
	}
	for _, u := range bad {
		if _, err := ValidateDiscordWebhookURL(u); err == nil {
			t.Errorf("expected %q invalid", u)
		}
	}
}

func TestValidateRoleID(t *testing.T) {
	if _, err := ValidateRoleID("123456789012345678"); err != nil {
		t.Errorf("valid role id rejected: %v", err)
	}
	for _, bad := range []string{"", "abc", "12", "12345678901234567890123456"} {
		if _, err := ValidateRoleID(bad); err == nil {
			t.Errorf("expected %q invalid", bad)
		}
	}
}

func TestValidateMachineIdentifier(t *testing.T) {
	if _, err := ValidateMachineIdentifier("6a646027a56abb6dbdf72484564db8567c737430"); err != nil {
		t.Errorf("valid machine id rejected: %v", err)
	}
	for _, bad := range []string{"", "has space", "bad/slash", "new\nline"} {
		if _, err := ValidateMachineIdentifier(bad); err == nil {
			t.Errorf("expected %q invalid", bad)
		}
	}
}

func TestCleanText(t *testing.T) {
	in := "hello\x00\x07world\nsecond\tline"
	out := CleanText(in, 100, false)
	if out != "helloworld\nsecond line" {
		t.Errorf("unexpected clean output: %q", out)
	}
	single := CleanText("a\nb", 100, true)
	if single != "a b" {
		t.Errorf("single line clean: %q", single)
	}
	if got := CleanText("abcdef", 3, false); got != "abc" {
		t.Errorf("truncate: %q", got)
	}
}

func TestRedactHint(t *testing.T) {
	if got := RedactHint(""); got != "" {
		t.Errorf("empty secret hint: %q", got)
	}
	if got := RedactHint("short"); got != "••••" {
		t.Errorf("short secret hint: %q", got)
	}
	if got := RedactHint("verysecretabcd"); got != "••••abcd" {
		t.Errorf("hint: %q", got)
	}
}
