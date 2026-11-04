// Part of the management plane's repository over the JumpServer API.
//
// The transport and the parsing of the two response shapes v5 mixes live in
// jms-kit/pkg/jmsapi; what is here is the questions the tools ask and the
// resources they get back.
package repository

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"

	"github.com/oekit/pkg/convert"
)

func (c *Client) SessionAssetIDs(ctx context.Context, ref string) ([]string, []map[string]any, error) {
	sessions, err := c.List(ctx, jmsapi.ResSessions)
	if err != nil {
		return nil, nil, err
	}
	low := strings.ToLower(strings.TrimSpace(ref))
	byID := map[string]map[string]string{}
	for _, s := range sessions {
		name := strings.ToLower(fmt.Sprint(jmsapi.AssetField(s, "name")))
		addr := strings.ToLower(fmt.Sprint(jmsapi.AssetField(s, "address")))
		if name != low && addr != low {
			continue
		}
		id := fmt.Sprint(jmsapi.AssetField(s, "id"))
		if id == "" || id == "<nil>" {
			continue
		}
		if _, ok := byID[id]; !ok {
			byID[id] = map[string]string{"name": name, "address": addr}
		}
	}
	ids := make([]string, 0, len(byID))
	meta := make([]map[string]any, 0, len(byID))
	for id, v := range byID {
		ids = append(ids, id)
		meta = append(meta, map[string]any{"id": id, "name": v["name"], "address": v["address"]})
	}
	sort.Strings(ids)
	sort.SliceStable(meta, func(i, j int) bool {
		return convert.AnyToStr(meta[i]["id"]) < convert.AnyToStr(meta[j]["id"])
	})
	return ids, meta, nil
}

func (c *Client) DownloadReplayText(ctx context.Context, sessionID string) (string, error) {
	// The REST helper marshals to JSON, so fetch the archive separately.
	req, err := http.NewRequestWithContext(ctx, "GET",
		c.BaseURL+"/api/v1"+string(jmsapi.ResSessions)+sessionID+"/replay/download/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return CastToText(raw)
}

func CastToText(tarBytes []byte) (string, error) {
	tr := tar.NewReader(bytes.NewReader(tarBytes))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read replay tar: %w", err)
		}
		if !strings.HasSuffix(hdr.Name, ".cast.gz") {
			continue
		}
		gz, err := io.ReadAll(tr)
		if err != nil {
			return "", fmt.Errorf("read cast: %w", err)
		}
		zr, err := gzip.NewReader(bytes.NewReader(gz))
		if err != nil {
			return "", fmt.Errorf("gunzip cast: %w", err)
		}
		defer zr.Close()
		raw, err := io.ReadAll(zr)
		if err != nil {
			return "", fmt.Errorf("read cast body: %w", err)
		}
		return AsciinemaToText(raw), nil
	}
	return "", fmt.Errorf("no .cast.gz member in the replay archive")
}

func AsciinemaToText(raw []byte) string {
	var b strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row []json.RawMessage
		if json.Unmarshal([]byte(line), &row) != nil || len(row) < 3 {
			continue
		}
		var kind, data string
		if json.Unmarshal(row[1], &kind) != nil || json.Unmarshal(row[2], &data) != nil {
			continue
		}
		if kind == "o" {
			b.WriteString(data)
		}
	}
	return b.String()
}
