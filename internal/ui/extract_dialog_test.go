package ui

import (
	"image"
	"image/color"
	"slices"
	"testing"

	"gioui.org/io/key"

	"world-kit/internal/screenshot"
	"world-kit/internal/settings"
)

func newOpenExtractDialog(lines ...chatLine) *extractDialog {
	d := newExtractDialog(func() {}, &memorySettings{})
	d.Open(lines)
	d.isEditing, d.editor.isEditable = true, true
	return d
}

func TestExtractDialogLinesApplyMarkup(t *testing.T) {
	d := newOpenExtractDialog(
		chatLine{copyText: "John Doe says: Hello *waves*", color: colorChatSay},
		chatLine{copyText: "* John Doe nods.", color: colorChatEmote},
	)
	want := []screenshot.Line{
		{Spans: []screenshot.Span{{Text: "John Doe says: Hello ", Color: colorChatSay}, {Text: "waves", Color: colorChatEmote}}, Color: colorChatSay},
		{Spans: []screenshot.Span{{Text: "* John Doe nods.", Color: colorChatEmote}}, Color: colorChatEmote},
	}
	got := d.Lines()
	if len(got) != len(want) {
		t.Fatalf("Lines() = %v, want %v", got, want)
	}
	for i := range want {
		if !slices.Equal(got[i].Spans, want[i].Spans) || got[i].Color != want[i].Color {
			t.Errorf("line %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestExtractDialogStyle(t *testing.T) {
	tests := []struct {
		backdrop int
		want     color.NRGBA
	}{
		{backdropTransparent, color.NRGBA{}},
		{backdropDark, colorBackdropDark},
		{backdropLight, colorBackdropLight},
	}
	for _, test := range tests {
		d := newExtractDialog(func() {}, &memorySettings{})
		d.backdrop.selected = test.backdrop
		if got := d.Style().Backdrop; got != test.want {
			t.Errorf("backdrop %d: Style().Backdrop = %v, want %v", test.backdrop, got, test.want)
		}
	}
}

// editorText is the editor's document as plain text lines.
func editorText(e *pictureEditor) []string {
	var texts []string
	for _, line := range e.doc.Lines() {
		text := ""
		for _, span := range line.Spans {
			text += span.Text
		}
		texts = append(texts, text)
	}
	return texts
}

func TestPictureEditorKeys(t *testing.T) {
	press := func(name key.Name, modifiers key.Modifiers) key.Event {
		return key.Event{Name: name, Modifiers: modifiers, State: key.Press}
	}
	tests := []struct {
		name  string
		start screenshot.Position
		keys  []key.Event
		typed string
		want  []string
	}{
		{"backspace", screenshot.Position{Index: 2}, []key.Event{press(key.NameDeleteBackward, 0)}, "", []string{"hllo", "world"}},
		{"delete forward", screenshot.Position{}, []key.Event{press(key.NameDeleteForward, 0)}, "", []string{"ello", "world"}},
		{"backspace at a line start joins lines", screenshot.Position{Line: 1}, []key.Event{press(key.NameDeleteBackward, 0)}, "", []string{"helloworld"}},
		{"type over a shift selection", screenshot.Position{}, []key.Event{press(key.NameRightArrow, key.ModShift), press(key.NameRightArrow, key.ModShift)}, "J", []string{"Jllo", "world"}},
		{"enter splits", screenshot.Position{Index: 2}, []key.Event{press(key.NameReturn, 0)}, "", []string{"he", "llo", "world"}},
		{"select all and type", screenshot.Position{}, []key.Event{press("A", key.ModShortcut)}, "x", []string{"x"}},
		{"undo", screenshot.Position{Index: 5}, []key.Event{press(key.NameDeleteBackward, 0), press("Z", key.ModShortcut)}, "", []string{"hello", "world"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := newOpenExtractDialog(chatLine{copyText: "hello", color: colorChatSay}, chatLine{copyText: "world", color: colorChatSay})
			e := &d.editor
			e.selection = screenshot.Selection{Anchor: test.start, Caret: test.start}
			for _, ev := range test.keys {
				e.handleKey(ev)
			}
			if test.typed != "" {
				e.replaceSelection(test.typed)
			}
			if got := editorText(e); !slices.Equal(got, test.want) {
				t.Errorf("text = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPictureEditorRestyle(t *testing.T) {
	d := newOpenExtractDialog(chatLine{copyText: "Jane waves", color: colorChatSay})
	e := &d.editor
	e.selection = screenshot.Selection{Caret: screenshot.Position{Index: 4}}
	e.Restyle(func(doc *screenshot.Document, sel screenshot.Selection) { doc.SetColor(sel, colorChatRed) })
	want := []screenshot.Span{{Text: "Jane", Color: colorChatRed}, {Text: " waves", Color: colorChatSay}}
	if got := e.doc.Lines()[0].Spans; !slices.Equal(got, want) {
		t.Errorf("spans = %v, want %v", got, want)
	}
	// Nothing selected: the tool changes all the text.
	e.selection = screenshot.Selection{}
	e.Restyle((*screenshot.Document).ToggleItalic)
	for _, span := range e.doc.Lines()[0].Spans {
		if !span.IsItalic {
			t.Errorf("span %q not italic", span.Text)
		}
	}
	if !e.undo() || e.doc.Lines()[0].Spans[0].IsItalic {
		t.Error("undo didn't restore the upright text")
	}
}

func TestFittedSize(t *testing.T) {
	tests := []struct {
		size     image.Point
		maxWidth int
		want     image.Point
	}{
		{image.Pt(400, 100), 600, image.Pt(400, 100)},
		{image.Pt(600, 100), 600, image.Pt(600, 100)},
		{image.Pt(900, 90), 600, image.Pt(600, 60)},
		{image.Point{}, 600, image.Point{}},
	}
	for _, test := range tests {
		if got := fittedSize(test.size, test.maxWidth); got != test.want {
			t.Errorf("fittedSize(%v, %d) = %v, want %v", test.size, test.maxWidth, got, test.want)
		}
	}
}

func TestExtractDialogStyleFontSize(t *testing.T) {
	d := newExtractDialog(func() {}, &memorySettings{})
	if got := d.Style().FontSize; got != screenshot.DefaultFontSize {
		t.Errorf("default Style().FontSize = %d, want %d", got, screenshot.DefaultFontSize)
	}
	d.fontSize.step(4)
	if got := d.Style().FontSize; got != screenshot.DefaultFontSize+4 {
		t.Errorf("Style().FontSize after +4 = %d, want %d", got, screenshot.DefaultFontSize+4)
	}
}

func TestNumberStepperStaysInRange(t *testing.T) {
	tests := []struct {
		start, delta, want int
	}{
		{14, 1, 15},
		{14, -1, 13},
		{24, 1, 24},
		{10, -1, 10},
		{12, 100, 24},
	}
	for _, test := range tests {
		s := newNumberStepper(test.start, 10, 24, "px")
		s.step(test.delta)
		if s.value != test.want {
			t.Errorf("%d%+d = %d, want %d", test.start, test.delta, s.value, test.want)
		}
	}
}

func TestNumberStepperLandsOnIncrements(t *testing.T) {
	tests := []struct {
		start, delta, want int
	}{
		{914, 1, 950},
		{914, -1, 900},
		{900, -1, 850},
		{900, 1, 950},
		{220, -1, 200}, // stays at the minimum
	}
	for _, test := range tests {
		s := &numberStepper{value: test.start, min: 200, max: 1800, increment: 50}
		s.step(test.delta)
		if s.value != test.want {
			t.Errorf("%d%+d = %d, want %d", test.start, test.delta, s.value, test.want)
		}
	}
}

func TestExtractWidthFollowsFontSizeUntilChosen(t *testing.T) {
	d := newExtractDialog(func() {}, &memorySettings{})
	if got := d.Style().WidthLimit; got != screenshot.DefaultWidthLimit(screenshot.DefaultFontSize) {
		t.Fatalf("default WidthLimit = %d, want the in-game width", got)
	}
	d.fontSize.step(4)
	d.followFontSize()
	if got, want := d.Style().WidthLimit, screenshot.DefaultWidthLimit(screenshot.DefaultFontSize+4); got != want {
		t.Errorf("after a bigger font, WidthLimit = %d, want %d", got, want)
	}
	d.widthLimit.value, d.hasChosenWidth = 500, true
	d.fontSize.step(-4)
	d.followFontSize()
	if got := d.Style().WidthLimit; got != 500 {
		t.Errorf("a chosen width changed with the font: %d, want 500", got)
	}
}

// memorySettings is a SettingsStore that keeps settings in memory, for tests.
type memorySettings struct {
	current settings.Settings
}

func (m *memorySettings) Get() settings.Settings { return m.current }

func (m *memorySettings) Update(change func(*settings.Settings)) error {
	change(&m.current)
	return nil
}

func TestExtractPresetsSaveAndApply(t *testing.T) {
	store := &memorySettings{}
	d := newExtractDialog(func() {}, store)
	d.backdrop.selected, d.fontSize.value = backdropDark, 18
	d.widthLimit.value, d.hasChosenWidth = 500, true
	store.current.SaveExtractPreset(d.currentPreset("Forum"))

	other := newExtractDialog(func() {}, store)
	other.applyPresetNamed("Forum")
	got := other.Style()
	if got.Backdrop != colorBackdropDark || got.FontSize != 18 || got.WidthLimit != 500 || other.presetName != "Forum" {
		t.Errorf("applied preset gives %+v (preset %q)", got, other.presetName)
	}
}

func TestExtractPresetKeepsFollowingTheFontSize(t *testing.T) {
	store := &memorySettings{}
	store.current.SaveExtractPreset(settings.ExtractPreset{Name: "Big", Backdrop: "light", FontSize: 22})
	store.current.DefaultExtractPreset = "Big"
	d := newExtractDialog(func() {}, store)
	d.Open([]chatLine{{copyText: "John Doe says: hi", color: colorChatSay}})
	if got, want := d.Style().WidthLimit, screenshot.DefaultWidthLimit(22); got != want || d.hasChosenWidth {
		t.Errorf("default preset without a width: WidthLimit = %d (chosen %v), want %d following the font", got, d.hasChosenWidth, want)
	}
	if preset := d.currentPreset("again"); preset.WidthLimit != 0 {
		t.Errorf("re-saved preset WidthLimit = %d, want 0 (follow the font)", preset.WidthLimit)
	}
}
