package internal

import "fmt"

type where int

// For given example: `validate:"required,oneof=red green blue,oneof=r g b"`.
// The entire expression is surrounded by backquotes ```.
// The first and last rune must be backquote and are described by begin and end.
// tagKey is the word `validate`
// tagSeparator is the colon `:< between `validate` and `"required“
// tagValue is the quoted value `"required,oneof=red green blue,oneof=r g b"` without end raw quote.
// The tag value is internally composed of either boolean attributes or key-value pairs.
// pairKey is the key of the key-value pair OR a boolean attribute, so both `oneof` and `required`.
// pairKeyValueSeparator is the equal sign `=` between pairKey and the pairValue.
// pairValue is the value of the key-value pair, so `red green blue` or `r g b`.
// pairSeparator is the comma `,` between two pairs.
// insideTagValueQuotes is the value without quotes `required,oneof=red green blue,oneof=r g b`
const (
	idk where = 1 << iota
	begin
	tagKey
	tagSeparator
	tagValue
	pairKey
	pairKeyValueSeparator
	pairValue
	pairSeparator
	end
)

type parseState struct {
	rawQuotes uint8
	at, next  where

	// skipTag is true when the tagKey is not 'validate'
	// value of such tag can be ignored
	skipTag bool

	buf []rune

	key  string
	vals Validations
}

func newParseState() *parseState {
	return &parseState{vals: make(Validations)}
}

// accept checks if the parseState can accept the rune c
// and if possible, accumulates it into the buf.
func (s *parseState) accept(c rune) error {
	if s.isAt(end) {
		return fmt.Errorf("already reached the end, got unexpected next rune")
	}

	switch c {
	case '`':
		return s.acceptRawQuote()
	case ' ':
		return s.acceptSpace()
	case ':':
		return s.acceptColon()
	case '"':
		return s.acceptQuote()
	case ',':
		return s.acceptComma()
	case '=':
		return s.acceptEqualSign()
	}

	if s.isAt(begin) {
		s.at = tagKey
		s.buf = append(s.buf, c)
		return nil
	}

	if s.isAt(tagKey) {
		s.buf = append(s.buf, c)
		return nil
	}

	if s.isAt(pairKey | pairValue) {
		s.buf = append(s.buf, c)
		return nil
	}

	if s.isAt(pairKeyValueSeparator) {
		s.setAt(pairValue)
		s.buf = append(s.buf, c)
		return nil
	}

	if s.isAt(pairSeparator) && s.nextIs(pairKey) {
		s.setAt(pairKey)
		s.setNext(idk)
		s.buf = append(s.buf, c)
		return nil
	}

	return nil
}

func (s *parseState) acceptSpace() error {
	switch {
	case s.isAt(begin):
		return nil
	case s.isAt(tagValue):
		s.buf = append(s.buf, ' ')
		return nil
	case s.isAt(tagKey):
		return fmt.Errorf("space in tag key not allowed %s", s)
	case s.isAt(tagSeparator):
		return fmt.Errorf("space not allowed after tagSeparator, expected quote '\"' after: %q", s.buf)
	case s.isAt(pairKey):
		s.setAt(begin)
		s.setNext(tagValue)
		s.storeKey()
		return nil
	case s.isAt(pairValue):
		s.buf = append(s.buf, ' ')
		return nil
	}

	return fmt.Errorf("unexpected space %s", s)
}

func (s *parseState) acceptRawQuote() error {
	switch s.rawQuotes {
	case 0:
		s.rawQuotes = 1
		s.setAt(begin)
		s.setNext(tagKey)
		return nil
	case 1:
		s.rawQuotes = 2
		s.setAt(end)
		s.setNext(idk)
		return nil
	}

	return fmt.Errorf("unexpected number of raw quotes: %d %s", s.rawQuotes, s)
}

func (s *parseState) String() string {
	return fmt.Sprintf("[parse state at: %d, next: %d, buf: %s, key: %s, vals: %v]", s.at, s.next, string(s.buf), string(s.key), s.vals)
}

func (s *parseState) acceptColon() error {
	if s.isAt(tagKey) {
		if string(s.buf) == "validate" {
			s.setAt(tagSeparator)
			s.setNext(tagValue)
		} else {
			s.skipTag = true
			s.setNext(tagValue)
		}

		s.buf = nil
		return nil
	}

	return fmt.Errorf("unexpected colon: %s", s)
}

func (s *parseState) acceptQuote() error {
	if s.nextIs(tagValue) {
		s.setAt(pairKey)
		s.setNext(pairKey)
		return nil
	}

	if s.skipTag {
		return nil
	}

	if s.isAt(pairKey) {
		s.setNext(pairKeyValueSeparator | tagValue)
		s.storeKey()
		return nil
	}

	if s.isAt(pairValue) {
		s.storeValue()
		return nil
	}

	return fmt.Errorf("unexpected quote after: %q, at: %d", s.buf, s.at)
}

func (s *parseState) acceptComma() error {
	if s.isAt(pairKey) {
		s.storeKey()
		s.setNext(pairKey)
		return nil
	}

	if s.isAt(pairValue) {
		s.storeValue()
		s.setNext(pairKey)
		s.setAt(pairSeparator)
		return nil
	}

	return fmt.Errorf("unexpected comma %s", s)
}

func (s *parseState) acceptEqualSign() error {
	if !s.isAt(pairKey) {
		return fmt.Errorf("unexpected equal sign %s", s)
	}

	s.storeKey()
	s.setAt(pairKeyValueSeparator)
	s.setNext(pairValue)
	return nil
}

func (s *parseState) storeKey() {
	if len(s.buf) == 0 {
		return
	}

	s.key = string(s.buf)
	s.vals[s.key] = append(s.vals[s.key], make([]string, 0)...)
	s.buf = nil
}

func (s *parseState) storeValue() {
	if len(s.buf) == 0 {
		return
	}

	s.vals[s.key] = append(s.vals[s.key], string(s.buf))
	s.buf = nil
}

func (s *parseState) isAt(w where) bool {
	return s.at&w != 0
}

func (s *parseState) nextIs(w where) bool {
	return s.next&w != 0
}

// smell of Java, because you can add debug logging here XD
func (s *parseState) setAt(w where) {
	s.at = w
}

func (s *parseState) setNext(w where) {
	s.next = w
}

func (s *parseState) build() (Validations, error) {
	return s.vals, nil
}
