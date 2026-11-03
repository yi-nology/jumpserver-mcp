// Part of the management plane's repository over the JumpServer API.
//
// The transport and the parsing of the two response shapes v5 mixes live in
// jms-kit/pkg/jmsapi; what is here is the questions the tools ask and the
// resources they get back.
package client

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/yi-nology/jumpserver-mcp-kit/pkg/jmsapi"

	"github.com/oekit/pkg/convert"
)

func (c *Client) LabelLookup(ctx context.Context, name, value string) (string, string, error) {
	q := url.Values{"name": {name}}
	if value != "" {
		q.Set("value", value)
	}
	rows, err := c.ListQuery(ctx, jmsapi.ResLabels, q)
	if err != nil {
		return "", "", err
	}
	var names []string
	for _, m := range rows {
		n := convert.AnyToStr(m["name"])
		names = append(names, n)
		if strings.EqualFold(n, name) && (value == "" || convert.AnyToStr(m["value"]) == value) {
			return convert.AnyToStr(m["id"]), convert.AnyToStr(m["value"]), nil
		}
	}
	if len(names) > 0 {
		return "", "", fmt.Errorf("no label %q with value %q; this key exists with: %s",
			name, value, strings.Join(names, ", "))
	}
	return "", "", fmt.Errorf("no label %q; list them with jms_labels_list", name)
}

func (c *Client) RoleLookup(ctx context.Context, ref string) (string, error) {
	if IsUUID(ref) {
		return ref, nil
	}
	rows, err := c.ListQuery(ctx, jmsapi.ResOrgRoles, url.Values{"name": {ref}})
	if err != nil {
		return "", err
	}
	var names []string
	for _, m := range rows {
		n := convert.AnyToStr(m["name"])
		names = append(names, n)
		if strings.EqualFold(n, ref) {
			return convert.AnyToStr(m["id"]), nil
		}
	}
	return "", fmt.Errorf("no role %q; available: %s", ref, strings.Join(names, ", "))
}

func (c *Client) ContentTypeID(ctx context.Context, model string) (int, error) {
	rows, err := c.List(ctx, jmsapi.ResContentTypes)
	if err != nil {
		return 0, err
	}
	var models []string
	for _, m := range rows {
		mo := convert.AnyToStr(m["model"])
		models = append(models, mo)
		if strings.EqualFold(mo, model) {
			if id, ok := m["id"].(float64); ok {
				return int(id), nil
			}
		}
	}
	sort.Strings(models)
	return 0, fmt.Errorf("unknown content type %q; known models include: %s",
		model, strings.Join(models, ", "))
}

// labelResourcePath is the endpoint that lists the resources a label covers.
func labelResourcePath(labelID string, contentTypeID int) string {
	return fmt.Sprintf("%s%s/resource-types/%d/resources/", string(jmsapi.ResLabels), labelID, contentTypeID)
}

func (c *Client) LabelAttach(ctx context.Context, labelID string, ctID int, add, remove []string) (string, error) {
	path := labelResourcePath(labelID, ctID)

	cur, err := c.LabelBoundIDs(ctx, labelID, ctID)
	if err != nil {
		return "", err
	}
	before := len(cur)
	desired := map[string]bool{}
	for _, id := range cur {
		desired[id] = true
	}
	for _, id := range add {
		desired[id] = true
	}
	for _, id := range remove {
		delete(desired, id)
	}
	want := slices.Collect(maps.Keys(desired))
	sort.Strings(want)

	body := map[string]any{"res_ids": want}
	var out map[string]any
	if err := c.Do(ctx, "PUT", path, nil, body, &out); err != nil {
		return "", fmt.Errorf("attach failed: %w", err)
	}
	return fmt.Sprintf("%d resource(s) now carry the label (was %d)", len(want), before), nil
}

// LabelBoundIDs reads the ids of the resources a label is bound to.
//
// It is the "bound=1" view: the resources that actually carry the label,
// as opposed to every resource of that content type.
func (c *Client) LabelBoundIDs(ctx context.Context, labelID string, ctID int) ([]string, error) {
	path := labelResourcePath(labelID, ctID)
	rows, err := c.ListAll(ctx, path+"?bound=1&page_size=500")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, m := range rows {
		if id, ok := m["id"]; ok && id != nil {
			out = append(out, convert.AnyToStr(id))
		}
	}
	return out, nil
}
