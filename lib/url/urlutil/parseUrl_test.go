package urlutil

import (
	"testing"
)

func TestFilePlain(t *testing.T) {
	pathname := "/dir0/dir1/file2"
	driverData, err := parseUrl(pathname)
	if err != nil {
		t.Fatal(err)
	}
	if driverFile != driverData.driverType {
		t.Fatalf("expected driverType:%d, got: %d",
			driverFile, driverData.driverType)
	}
	if pathname != driverData.pathname {
		t.Fatalf("expected pathname: \"%s\", got: \"%s\"",
			pathname, driverData.pathname)
	}
}

func TestFileUrl(t *testing.T) {
	pathname := "/dir0/dir1/file2"
	driverData, err := parseUrl("file://" + pathname)
	if err != nil {
		t.Fatal(err)
	}
	if driverFile != driverData.driverType {
		t.Fatalf("expected driverType:%d, got: %d",
			driverFile, driverData.driverType)
	}
	if pathname != driverData.pathname {
		t.Fatalf("expected pathname: \"%s\", got: \"%s\"",
			pathname, driverData.pathname)
	}
}

func TestGitHTTP(t *testing.T) {
	pathname := "dir0/dir1/file2"
	repo := "http://github.com/Cloud-Foundations/bogus.git"
	driverData, err := parseUrl(repo + "?" + pathname)
	if err != nil {
		t.Fatal(err)
	}
	if driverGit != driverData.driverType {
		t.Fatalf("expected driverType:%d, got: %d",
			driverGit, driverData.driverType)
	}
	if repo != driverData.gitUrl {
		t.Fatalf("expected repo: \"%s\", got: \"%s\"",
			repo, driverData.gitUrl)
	}
	if pathname != driverData.pathname {
		t.Fatalf("expected pathname: \"%s\", got: \"%s\"",
			pathname, driverData.pathname)
	}
}

func TestGitHttpIncomplete(t *testing.T) {
	repo := "http://github.com/Cloud-Foundations/bogus.git?"
	_, err := parseUrl(repo)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGitHttpMissing(t *testing.T) {
	repo := "http://github.com/Cloud-Foundations/bogus.git"
	_, err := parseUrl(repo)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGitSSH(t *testing.T) {
	pathname := "dir0/dir1/file2"
	repo := "git@github.com:Cloud-Foundations/bogus.git"
	driverData, err := parseUrl(repo + "/" + pathname)
	if err != nil {
		t.Fatal(err)
	}
	if driverGit != driverData.driverType {
		t.Fatalf("expected driverType:%d, got: %d",
			driverGit, driverData.driverType)
	}
	if repo != driverData.gitUrl {
		t.Fatalf("expected repo: \"%s\", got: \"%s\"",
			repo, driverData.gitUrl)
	}
	if pathname != driverData.pathname {
		t.Fatalf("expected pathname: \"%s\", got: \"%s\"",
			pathname, driverData.pathname)
	}
}

func TestGitSshIncomplete(t *testing.T) {
	repo := "git@github.com:Cloud-Foundations/bogus.git/"
	_, err := parseUrl(repo)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGitSshMissing(t *testing.T) {
	repo := "git@github.com:Cloud-Foundations/bogus.git"
	_, err := parseUrl(repo)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestHTTP(t *testing.T) {
	url := "http://www.website.com/dir0/dir1/file2"
	driverData, err := parseUrl(url)
	if err != nil {
		t.Fatal(err)
	}
	if driverHttp != driverData.driverType {
		t.Fatalf("expected driverType:%d, got: %d",
			driverHttp, driverData.driverType)
	}
	if url != driverData.rawUrl {
		t.Fatalf("expected URL: \"%s\", got: \"%s\"",
			url, driverData.rawUrl)
	}
}
