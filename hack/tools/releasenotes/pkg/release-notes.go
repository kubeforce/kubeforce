package pkg

type ReleaseNotes struct {
	NoteGroups    []*NoteGroup
	UnsortedNotes []Note
}

type NoteGroup struct {
	Title string
	Notes []Note
}

type Note struct {
	Subject string
	Body    string
	Refs    []string
}
