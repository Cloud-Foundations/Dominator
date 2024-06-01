package filegen

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/memory"
)

type urlGenerator struct {
	logger          log.Logger
	notifierChannel chan<- string
	objectServer    *memory.ObjectServer
	fullUrl         string
}

func (m *Manager) registerUrlForPath(pathname, URL string) {
	urlGen := &urlGenerator{
		logger:       m.logger,
		objectServer: m.objectServer,
		fullUrl:      URL + "?pathname=" + url.QueryEscape(pathname),
	}
	urlGen.notifierChannel = m.RegisterGeneratorForPath(pathname, urlGen)
}

func (urlGen *urlGenerator) Generate(machine mdb.Machine,
	logger log.Logger) ([]byte, time.Time, error) {
	requestBody := &bytes.Buffer{}
	if err := json.WriteWithIndent(requestBody, "    ", machine); err != nil {
		return nil, time.Time{}, err
	}
	resp, err := http.Post(urlGen.fullUrl, "application/json", requestBody)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, time.Time{}, errors.New(resp.Status)
	}
	var result programmeResult
	if err := json.Read(resp.Body, &result); err != nil {
		return nil, time.Time{}, err
	}
	var validUntil time.Time
	if result.SecondsValid > 0 {
		validUntil = time.Now().Add(time.Duration(result.SecondsValid) *
			time.Second)
	}
	return result.Data, validUntil, nil
}
