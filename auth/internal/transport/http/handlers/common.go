package handlers

type H map[string]interface{}

func throw(e string) *H {
	return &H{"error": e}
}
