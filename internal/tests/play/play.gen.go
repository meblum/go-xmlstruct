// This file is automatically generated. DO NOT EDIT.

package play

type Act struct {
	Epilogue Epilogue `xml:"EPILOGUE"`
	Scene    []Scene  `xml:"SCENE"`
	Title    string   `xml:"TITLE"`
}

type Epilogue struct {
	Speech         Speech `xml:"SPEECH"`
	StageDirection string `xml:"STAGEDIR"`
	Title          string `xml:"TITLE"`
}

type FrontMatter struct {
	Paragraph []string `xml:"P"`
}

type Line struct {
	CharData       string  `xml:",chardata"`
	StageDirection *string `xml:"STAGEDIR"`
}

type PersonaGroup struct {
	GroupDescription string   `xml:"GRPDESCR"`
	Persona          []string `xml:"PERSONA"`
}

type Personae struct {
	Persona      []string       `xml:"PERSONA"`
	PersonaGroup []PersonaGroup `xml:"PGROUP"`
	Title        string         `xml:"TITLE"`
}

type Play struct {
	Act               []Act       `xml:"ACT"`
	FrontMatter       FrontMatter `xml:"FM"`
	Personae          Personae    `xml:"PERSONAE"`
	PlaySubtitle      string      `xml:"PLAYSUBT"`
	ScreenDescription string      `xml:"SCNDESCR"`
	Title             string      `xml:"TITLE"`
}

type Scene struct {
	Speech         []Speech `xml:"SPEECH"`
	StageDirection []string `xml:"STAGEDIR"`
	Title          string   `xml:"TITLE"`
}

type Speech struct {
	Line           []Line    `xml:"LINE"`
	Speaker        string    `xml:"SPEAKER"`
	StageDirection []*string `xml:"STAGEDIR"`
}
