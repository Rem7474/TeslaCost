package handlers

import "testing"

func TestDocumentMimeTypeComesFromTheContent(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpeg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF")
	pdf := []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	webp := []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")

	accepted := map[string][]byte{"application/pdf": pdf, "image/png": png, "image/jpeg": jpeg, "image/webp": webp}
	for want, data := range accepted {
		if got, ok := documentMimeType(data); !ok || got != want {
			t.Errorf("%s: got (%q, %v)", want, got, ok)
		}
	}

	rejected := map[string][]byte{
		"html named receipt.pdf":      []byte("<!DOCTYPE html><html><script>alert(1)</script></html>"),
		"svg with script":             []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"plain text":                  []byte("just some notes"),
		"gif is not an accepted type": []byte("GIF89a\x01\x00\x01\x00"),
		"empty":                       nil,
	}
	for name, data := range rejected {
		if got, ok := documentMimeType(data); ok {
			t.Errorf("%s accepted as %q", name, got)
		}
	}
}
