package storage

// Event описывает событие (BIRT, DEAT) согласно GEDCOM 7.0
type Event struct {
	Date  string
	Place string
}

// Individual — это наш основной объект
type Individual struct {
	ID    string
	Name  string
	Birth *Event // Указатель: может быть nil, если данных нет
	Death *Event // Та самая "дата смерти"
}

// Store — хранилище всех предков
type Store struct {
	People map[string]*Individual
}

func NewStore() *Store {
	return &Store{People: make(map[string]*Individual)}
}