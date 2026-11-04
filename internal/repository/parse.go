package repository

// The matching semantics: how a name, an address or an id a human typed is
// resolved against what the bastion holds.
//
// The wire format itself — the shapes of the responses — lives in
// jms-kit/pkg/jmsapi. What is here is the rule about which row a reference
// means, and that rule belongs to the management plane: it is the tools that
// accept "192.0.2.71" from an agent, and only they need to decide what such
// a string may refer to.

import (
	"fmt"
	"strings"

	"github.com/oekit/pkg/convert"
	"github.com/yi-nology/jumpserver-kit/pkg/render"

	"github.com/yi-nology/jumpserver-kit/pkg/jmsapi"
)

// AssetMatcher matches a row against a set of assets the caller named.
//
// The bastion identifies an asset by id, but a person (and an agent) names it
// by address or by name, and the two are not the same string. The matcher
// holds both, so a filter that was written against either keeps working.
type AssetMatcher struct {
	ids   map[string]bool
	names map[string]bool
	addrs map[string]bool
}

// NewAssetMatcher builds a matcher for a set of resolved assets.
func NewAssetMatcher(ids []string, meta []map[string]any) *AssetMatcher {
	m := &AssetMatcher{ids: map[string]bool{}, names: map[string]bool{}, addrs: map[string]bool{}}
	for _, id := range ids {
		m.ids[strings.ToLower(id)] = true
	}
	for _, a := range meta {
		if n, ok := a["name"].(string); ok && n != "" {
			m.names[strings.ToLower(n)] = true
		}
		if ad, ok := a["address"].(string); ok && ad != "" {
			m.addrs[strings.ToLower(ad)] = true
		}
	}
	return m
}

// Match reports whether a row is one of the assets in the matcher.
func (m *AssetMatcher) Match(row map[string]any) bool {
	if m == nil {
		return true
	}
	if id, ok := jmsapi.AssetField(row, "id").(string); ok && id != "" {
		return m.ids[strings.ToLower(id)]
	}
	return m.MatchNameAddress(fmt.Sprint(jmsapi.AssetField(row, "name")), fmt.Sprint(jmsapi.AssetField(row, "address")))
}

// MatchNameAddress matches on the name or the address.
func (m *AssetMatcher) MatchNameAddress(name, address string) bool {
	if m == nil {
		return true
	}
	if name != "" && m.names[strings.ToLower(name)] {
		return true
	}
	if address != "" && m.addrs[strings.ToLower(address)] {
		return true
	}
	return false
}

// MatchAddress matches on the address alone.
//
// It exists for the callers where the ADDRESS is the identity and the name
// is only a snapshot: a trail record carries the name the asset had when
// the transfer happened, and a machine that is renamed afterwards would
// otherwise have its history attributed to whatever carries that name
// today.
//
// ⚠️ It is an ADDITION, not a change: MatchNameAddress still matches on
// either half, because most callers want that. The callers that want the
// stricter rule ask for this one.
func (m *AssetMatcher) MatchAddress(address string) bool {
	if m == nil {
		return true
	}
	if address == "" {
		return false
	}
	return m.addrs[strings.ToLower(address)]
}

// UserMatches reports whether a display string matches a filter.
//
// The bastion writes a user as "name(username)"; a filter written against
// either half has to match, because the two are both things a person types.
func UserMatches(display, filter string) bool {
	if filter == "" {
		return true
	}
	d := strings.ToLower(strings.TrimSpace(display))
	f := strings.ToLower(strings.TrimSpace(filter))
	if d == f {
		return true
	}
	for _, part := range strings.Split(d, "(") {
		if strings.TrimSuffix(strings.TrimSpace(part), ")") == f {
			return true
		}
	}
	return false
}

// NormLimit clamps a limit argument to what the endpoint will accept.
//
// ⚠️ 实现只有底座 `render.NormLimit` 一份，这里只是指路：曾经两份逐字拷贝并存
// （一份拷贝会漂），语义以底座为准。
func NormLimit(v, def, max int) int {
	return render.NormLimit(v, def, max)
}

// IsUUID reports whether a string has the shape of a JumpServer id.
func IsUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if r == '-' {
				return false
			}
		}
	}
	return true
}

// MatchesRef reports whether an object is the one a reference names.
//
// A reference may be an id, an exact name, or a fragment of one. The id is
// matched exactly; the name loosely, because that is what a person types
// when they do not know the exact spelling.
func MatchesRef(obj map[string]any, ref string) bool {
	if IsUUID(ref) {
		return strings.EqualFold(convert.AnyToStr(obj["id"]), ref)
	}
	if strings.EqualFold(convert.AnyToStr(obj["name"]), ref) {
		return true
	}
	return strings.Contains(strings.ToLower(convert.AnyToStr(obj["name"])), strings.ToLower(ref))
}
