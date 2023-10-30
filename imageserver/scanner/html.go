package scanner

import (
	"fmt"
	"io"
)

func (imdb *ImageDataBase) writeHtml(writer io.Writer) {
	imdb.lockWatcher.WriteHtml(writer, "ImageServer: ")
	fmt.Fprintf(writer,
		"Number of  <a href=\"listImages?output=text\">images</a>: "+
			"<a href=\"listImages\">%d</a><br>\n",
		imdb.CountImages())
	fmt.Fprintf(writer,
		"Number of  <a href=\"listDirectories?output=text\">directories</a>: "+
			"<a href=\"listDirectories\">%d</a><br>\n",
		imdb.CountDirectories())
}
