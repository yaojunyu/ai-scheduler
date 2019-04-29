package framework


type Action interface {
	// Name return the unique name of Action
	Name() string

	// Execute execute the main logic of action
	Execute(session *Session)
}

type Plugin interface {
	// Name return the unique name of Plugin
	Name() string

	// OnSessionOpen do some init work when session open
	OnSessionOpen(session *Session)
	// OnSessionClose do some clean work when session close
	OnSessionClose(session *Session)
}

type Framework interface {
	OpenSession() *Session
	CloseSession(*Session)
}