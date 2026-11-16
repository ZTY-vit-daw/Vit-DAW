package instruments

import (
	"sort"
	"strings"
)

// GMProgram is one entry of the General MIDI Level 1 instrument patch map
// (MMA "GM 1 Sound Set"). Number uses the human 1-128 display convention —
// the same convention as the melody-drum-v1 template manifest
// (gm_program 82 = "Lead 2 (sawtooth)"), so the two stay comparable. The
// wire program-change byte is Number-1.
type GMProgram struct {
	Number int
	Name   string
	Family string
}

func (p GMProgram) ProgramChange() int { return p.Number - 1 }

// gmProgramTable is the GM Level 1 patch map, verbatim ordering 1..128.
var gmProgramTable = []GMProgram{
	{1, "Acoustic Grand Piano", "Piano"},
	{2, "Bright Acoustic Piano", "Piano"},
	{3, "Electric Grand Piano", "Piano"},
	{4, "Honky-tonk Piano", "Piano"},
	{5, "Electric Piano 1", "Piano"},
	{6, "Electric Piano 2", "Piano"},
	{7, "Harpsichord", "Piano"},
	{8, "Clavinet", "Piano"},
	{9, "Celesta", "Chromatic Percussion"},
	{10, "Glockenspiel", "Chromatic Percussion"},
	{11, "Music Box", "Chromatic Percussion"},
	{12, "Vibraphone", "Chromatic Percussion"},
	{13, "Marimba", "Chromatic Percussion"},
	{14, "Xylophone", "Chromatic Percussion"},
	{15, "Tubular Bells", "Chromatic Percussion"},
	{16, "Dulcimer", "Chromatic Percussion"},
	{17, "Drawbar Organ", "Organ"},
	{18, "Percussive Organ", "Organ"},
	{19, "Rock Organ", "Organ"},
	{20, "Church Organ", "Organ"},
	{21, "Reed Organ", "Organ"},
	{22, "Accordion", "Organ"},
	{23, "Harmonica", "Organ"},
	{24, "Tango Accordion", "Organ"},
	{25, "Acoustic Guitar (nylon)", "Guitar"},
	{26, "Acoustic Guitar (steel)", "Guitar"},
	{27, "Electric Guitar (jazz)", "Guitar"},
	{28, "Electric Guitar (clean)", "Guitar"},
	{29, "Electric Guitar (muted)", "Guitar"},
	{30, "Overdriven Guitar", "Guitar"},
	{31, "Distortion Guitar", "Guitar"},
	{32, "Guitar Harmonics", "Guitar"},
	{33, "Acoustic Bass", "Bass"},
	{34, "Electric Bass (finger)", "Bass"},
	{35, "Electric Bass (pick)", "Bass"},
	{36, "Fretless Bass", "Bass"},
	{37, "Slap Bass 1", "Bass"},
	{38, "Slap Bass 2", "Bass"},
	{39, "Synth Bass 1", "Bass"},
	{40, "Synth Bass 2", "Bass"},
	{41, "Violin", "Strings"},
	{42, "Viola", "Strings"},
	{43, "Cello", "Strings"},
	{44, "Contrabass", "Strings"},
	{45, "Tremolo Strings", "Strings"},
	{46, "Pizzicato Strings", "Strings"},
	{47, "Orchestral Harp", "Strings"},
	{48, "Timpani", "Strings"},
	{49, "String Ensemble 1", "Ensemble"},
	{50, "String Ensemble 2", "Ensemble"},
	{51, "Synth Strings 1", "Ensemble"},
	{52, "Synth Strings 2", "Ensemble"},
	{53, "Choir Aahs", "Ensemble"},
	{54, "Voice Oohs", "Ensemble"},
	{55, "Synth Voice", "Ensemble"},
	{56, "Orchestra Hit", "Ensemble"},
	{57, "Trumpet", "Brass"},
	{58, "Trombone", "Brass"},
	{59, "Tuba", "Brass"},
	{60, "Muted Trumpet", "Brass"},
	{61, "French Horn", "Brass"},
	{62, "Brass Section", "Brass"},
	{63, "Synth Brass 1", "Brass"},
	{64, "Synth Brass 2", "Brass"},
	{65, "Soprano Sax", "Reed"},
	{66, "Alto Sax", "Reed"},
	{67, "Tenor Sax", "Reed"},
	{68, "Baritone Sax", "Reed"},
	{69, "Oboe", "Reed"},
	{70, "English Horn", "Reed"},
	{71, "Bassoon", "Reed"},
	{72, "Clarinet", "Reed"},
	{73, "Piccolo", "Pipe"},
	{74, "Flute", "Pipe"},
	{75, "Recorder", "Pipe"},
	{76, "Pan Flute", "Pipe"},
	{77, "Blown Bottle", "Pipe"},
	{78, "Shakuhachi", "Pipe"},
	{79, "Whistle", "Pipe"},
	{80, "Ocarina", "Pipe"},
	{81, "Lead 1 (square)", "Synth Lead"},
	{82, "Lead 2 (sawtooth)", "Synth Lead"},
	{83, "Lead 3 (calliope)", "Synth Lead"},
	{84, "Lead 4 (chiff)", "Synth Lead"},
	{85, "Lead 5 (charang)", "Synth Lead"},
	{86, "Lead 6 (voice)", "Synth Lead"},
	{87, "Lead 7 (fifths)", "Synth Lead"},
	{88, "Lead 8 (bass + lead)", "Synth Lead"},
	{89, "Pad 1 (new age)", "Synth Pad"},
	{90, "Pad 2 (warm)", "Synth Pad"},
	{91, "Pad 3 (polysynth)", "Synth Pad"},
	{92, "Pad 4 (choir)", "Synth Pad"},
	{93, "Pad 5 (bowed)", "Synth Pad"},
	{94, "Pad 6 (metallic)", "Synth Pad"},
	{95, "Pad 7 (halo)", "Synth Pad"},
	{96, "Pad 8 (sweep)", "Synth Pad"},
	{97, "FX 1 (rain)", "Synth Effects"},
	{98, "FX 2 (soundtrack)", "Synth Effects"},
	{99, "FX 3 (crystal)", "Synth Effects"},
	{100, "FX 4 (atmosphere)", "Synth Effects"},
	{101, "FX 5 (brightness)", "Synth Effects"},
	{102, "FX 6 (goblins)", "Synth Effects"},
	{103, "FX 7 (echoes)", "Synth Effects"},
	{104, "FX 8 (sci-fi)", "Synth Effects"},
	{105, "Sitar", "Ethnic"},
	{106, "Banjo", "Ethnic"},
	{107, "Shamisen", "Ethnic"},
	{108, "Koto", "Ethnic"},
	{109, "Kalimba", "Ethnic"},
	{110, "Bag Pipe", "Ethnic"},
	{111, "Fiddle", "Ethnic"},
	{112, "Shanai", "Ethnic"},
	{113, "Tinkle Bell", "Percussive"},
	{114, "Agogo", "Percussive"},
	{115, "Steel Drums", "Percussive"},
	{116, "Woodblock", "Percussive"},
	{117, "Taiko Drum", "Percussive"},
	{118, "Melodic Tom", "Percussive"},
	{119, "Synth Drum", "Percussive"},
	{120, "Reverse Cymbal", "Percussive"},
	{121, "Guitar Fret Noise", "Sound Effects"},
	{122, "Breath Noise", "Sound Effects"},
	{123, "Seashore", "Sound Effects"},
	{124, "Bird Tweet", "Sound Effects"},
	{125, "Telephone Ring", "Sound Effects"},
	{126, "Helicopter", "Sound Effects"},
	{127, "Applause", "Sound Effects"},
	{128, "Gunshot", "Sound Effects"},
}

// gmPercussionKeyMap is the GM Level 1 percussion key map (channel 10),
// notes 35..81. The melody-drum-v1 percussion notation keys
// (36 kick / 38 snare / 42 closed_hat) resolve through this map.
var gmPercussionKeyMap = map[int]string{
	35: "Acoustic Bass Drum", 36: "Electric Bass Drum", 37: "Side Stick",
	38: "Acoustic Snare", 39: "Hand Clap", 40: "Electric Snare",
	41: "Low Floor Tom", 42: "Closed Hi-Hat", 43: "High Floor Tom",
	44: "Pedal Hi-Hat", 45: "Low Tom", 46: "Open Hi-Hat",
	47: "Low-Mid Tom", 48: "High-Mid Tom", 49: "Crash Cymbal 1",
	50: "High Tom", 51: "Ride Cymbal 1", 52: "Chinese Cymbal",
	53: "Ride Bell", 54: "Tambourine", 55: "Splash Cymbal",
	56: "Cowbell", 57: "Crash Cymbal 2", 58: "Vibraslap",
	59: "Ride Cymbal 2", 60: "High Bongo", 61: "Low Bongo",
	62: "Mute High Conga", 63: "Open High Conga", 64: "Low Conga",
	65: "High Timbale", 66: "Low Timbale", 67: "High Agogo",
	68: "Low Agogo", 69: "Cabasa", 70: "Maracas",
	71: "Short Whistle", 72: "Long Whistle", 73: "Short Guiro",
	74: "Long Guiro", 75: "Claves", 76: "High Woodblock",
	77: "Low Woodblock", 78: "Mute Cuica", 79: "Open Cuica",
	80: "Mute Triangle", 81: "Open Triangle",
}

// GMPrograms returns the full GM Level 1 patch map in number order.
func GMPrograms() []GMProgram {
	out := make([]GMProgram, len(gmProgramTable))
	copy(out, gmProgramTable)
	return out
}

// LookupGMProgram resolves a GM program by its 1-128 display number.
func LookupGMProgram(number int) (GMProgram, bool) {
	if number < 1 || number > len(gmProgramTable) {
		return GMProgram{}, false
	}
	return gmProgramTable[number-1], true
}

// LookupGMProgramByName resolves a GM program from free text by normalized
// containment against the official names (for example a textbook score
// annotation "Lead 2 (sawtooth)" or "Acoustic Grand Piano"). Longer official
// names win so "Electric Grand Piano" beats "Grand Piano" containment.
func LookupGMProgramByName(query string) (GMProgram, bool) {
	q := normalizeGMName(query)
	if q == "" {
		return GMProgram{}, false
	}
	var best GMProgram
	found := false
	for _, program := range gmProgramTable {
		name := normalizeGMName(program.Name)
		if name == "" {
			continue
		}
		if strings.Contains(q, name) || strings.Contains(name, q) && tokenCount(q) >= 2 {
			if !found || len(name) > len(normalizeGMName(best.Name)) {
				best = program
				found = true
			}
		}
	}
	return best, found
}

// GMPercussionKey resolves one GM percussion key (channel 10 note number).
func GMPercussionKey(note int) (string, bool) {
	name, ok := gmPercussionKeyMap[note]
	return name, ok
}

// GMPercussionKeys returns the full GM percussion key map in note order.
func GMPercussionKeys() map[int]string {
	out := make(map[int]string, len(gmPercussionKeyMap))
	for note, name := range gmPercussionKeyMap {
		out[note] = name
	}
	return out
}

// GMPercussionNotes returns the mapped note numbers in ascending order.
func GMPercussionNotes() []int {
	out := make([]int, 0, len(gmPercussionKeyMap))
	for note := range gmPercussionKeyMap {
		out = append(out, note)
	}
	sort.Ints(out)
	return out
}

func normalizeGMName(value string) string {
	replacer := strings.NewReplacer("(", " ", ")", " ", "-", " ", "_", " ")
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(replacer.Replace(value))))
	return strings.Join(fields, " ")
}

func tokenCount(value string) int {
	return len(strings.Fields(value))
}
