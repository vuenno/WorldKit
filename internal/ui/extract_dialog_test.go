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

func TestExtractDialogStyleLook(t *testing.T) {
	d := newExtractDialog(func() {}, &memorySettings{})
	if got, want := d.Style().Look, (screenshot.Look{Font: screenshot.FontArial, ChatScale: screenshot.DefaultChatScale}); got != want {
		t.Errorf("default look = %v, want %v", got, want)
	}
	d.chatFont.selected = chatFontIndex(screenshot.FontMukta)
	d.chatScale.step(-1)
	want := screenshot.Look{Font: screenshot.FontMukta, ChatScale: screenshot.DefaultChatScale - screenshot.ChatScaleStep}
	if got := d.Style().Look; got != want {
		t.Errorf("look after picking Mukta and a step down = %v, want %v", got, want)
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

func TestExtractWidthStartsAtTheInGameWidth(t *testing.T) {
	d := newExtractDialog(func() {}, &memorySettings{})
	if got := d.Style().WidthLimit; got != screenshot.DefaultWidthLimit() {
		t.Fatalf("default WidthLimit = %d, want the in-game width %d", got, screenshot.DefaultWidthLimit())
	}
	if preset := d.currentPreset("x"); preset.WidthLimit != 0 {
		t.Errorf("preset at the in-game width saves WidthLimit %d, want 0", preset.WidthLimit)
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
	d.backdrop.selected, d.chatFont.selected, d.chatScale.value = backdropDark, chatFontIndex(screenshot.FontMukta), 75
	d.widthLimit.value = 500
	store.current.SaveExtractPreset(d.currentPreset("Forum"))

	other := newExtractDialog(func() {}, store)
	other.applyPresetNamed("Forum")
	got := other.Style()
	wantLook := screenshot.Look{Font: screenshot.FontMukta, ChatScale: 75}
	if got.Backdrop != colorBackdropDark || got.Look != wantLook || got.WidthLimit != 500 || other.presetName != "Forum" {
		t.Errorf("applied preset gives %+v (preset %q)", got, other.presetName)
	}
}

func TestOldExtractPresetGetsTheNearestChatScale(t *testing.T) {
	store := &memorySettings{}
	// Saved before chat looks: a pixel font size, no font, no width.
	store.current.SaveExtractPreset(settings.ExtractPreset{Name: "Big", Backdrop: "light", FontSize: 22})
	store.current.DefaultExtractPreset = "Big"
	d := newExtractDialog(func() {}, store)
	d.Open([]chatLine{{copyText: "John Doe says: hi", color: colorChatSay}})
	want := screenshot.Look{Font: screenshot.FontArial, ChatScale: screenshot.ChatScaleForFontSize(22)}
	if got := d.Style(); got.Look != want || got.WidthLimit != screenshot.DefaultWidthLimit() {
		t.Errorf("old preset gives %+v, want %v at the in-game width", got, want)
	}
	if preset := d.currentPreset("again"); preset.FontSize != 0 || preset.ChatScale != want.ChatScale || preset.WidthLimit != 0 {
		t.Errorf("re-saved preset = %+v, want the chat scale, no font size, in-game width", preset)
	}
}
