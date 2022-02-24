package app

type DTOInstance struct {
	err     error
	content string
}

func NewDTOInstance(err error, content string) *DTOInstance {
	return &DTOInstance{
		err:     err,
		content: content,
	}
}

func (r *DTOInstance) Err() error {
	return r.err
}

func (r *DTOInstance) Content() string {
	return r.content
}

type DTO interface {
	Err() error
	Content() string
}
