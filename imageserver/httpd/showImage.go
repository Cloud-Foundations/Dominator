package httpd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/json"
)

var timeFormat string = "02 Jan 2006 15:04:05.99 MST"

func (s state) showImageHandler(w http.ResponseWriter, req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	imageName := req.URL.RawQuery
	fmt.Fprintf(writer, "<title>image %s</title>\n", imageName)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<h3>")
	img := s.imageDataBase.GetImage(imageName)
	if img == nil {
		fmt.Fprintf(writer, "Image: %s UNKNOWN!\n", imageName)
		return
	}
	checksum := s.imageDataBase.GetImageFileChecksum(imageName)
	usageEstimate, _ := s.imageDataBase.GetImageUsageEstimate(imageName)
	fmt.Fprintf(writer, "Information for image: %s<br>\n", imageName)
	fmt.Fprintln(writer, "</h3>")
	fmt.Fprintf(writer, "Data size: <a href=\"listImage?%s\">%s</a>",
		imageName, format.FormatBytes(img.FileSystem.TotalDataBytes))
	fmt.Fprintf(writer, " (%s estimated usage when unpacked)<br>\n",
		format.FormatBytes(usageEstimate))
	fmt.Fprintf(writer, "Number of data inodes: %d<br>\n",
		img.FileSystem.NumRegularInodes)
	if numInodes := img.FileSystem.NumComputedRegularInodes(); numInodes > 0 {
		fmt.Fprintf(writer,
			"Number of computed inodes: <a href=\"listComputedInodes?%s\">%d</a><br>\n",
			imageName, numInodes)
	}
	if img.Filter == nil {
		fmt.Fprintln(writer, "Image has no filter: sparse image<br>")
	} else if len(img.Filter.FilterLines) < 1 {
		fmt.Fprintln(writer,
			"Filter has 0 lines (empty filter: full coverage)<br>")
	} else {
		fmt.Fprintf(writer,
			"Filter has <a href=\"listFilter?%s\">%d</a> lines<br>\n",
			imageName, len(img.Filter.FilterLines))
	}
	if img.Triggers == nil || len(img.Triggers.Triggers) < 1 {
		fmt.Fprintln(writer, "Image has no triggers<br>")
	} else {
		fmt.Fprintf(writer,
			"Number of triggers: <a href=\"listTriggers?%s\">%d</a><br>\n",
			imageName, len(img.Triggers.Triggers))
	}
	if !img.ExpiresAt.IsZero() {
		fmt.Fprintf(writer, "Expires at: %s (in %s)<br>\n",
			img.ExpiresAt.In(time.Local).Format(timeFormat),
			format.Duration(time.Until(img.ExpiresAt)))
	}
	showAnnotation(writer, img.ReleaseNotes, imageName, "Release notes",
		"listReleaseNotes")
	showAnnotation(writer, img.BuildLog, imageName, "Build log",
		"listBuildLog")
	if img.CreatedBy != "" {
		fmt.Fprintf(writer, "Created by: %s\n<br>", img.CreatedBy)
	}
	if img.CreatedFor != "" {
		fmt.Fprintf(writer, "Created for: %s\n<br>", img.CreatedFor)
	}
	if !img.CreatedOn.IsZero() {
		fmt.Fprintf(writer, "Created on: %s (%s old)\n<br>",
			img.CreatedOn.In(time.Local).Format(timeFormat),
			format.Duration(time.Since(img.CreatedOn)))
	}
	if len(img.BuildGitUrl) > 0 {
		fmt.Fprintf(writer,
			"Built from Git repository: %s on branch: %s at commit: %s<br>\n",
			img.BuildGitUrl, img.BuildBranch, img.BuildCommitId)
	}
	if len(img.Packages) > 0 {
		fmt.Fprintf(writer,
			"Packages: <a href=\"listPackages?%s\">%d</a><br>\n",
			imageName, len(img.Packages))
	}
	if img.SourceImage != "" {
		if s.imageDataBase.CheckImage(img.SourceImage) {
			fmt.Fprintf(writer,
				"Source Image: <a href=\"showImage?%s\">%s</a><br>\n",
				img.SourceImage, img.SourceImage)
		} else {
			fmt.Fprintf(writer, "Source Image: %s</a><br>\n", img.SourceImage)
		}
	}
	if len(img.Tags) > 0 {
		fmt.Fprintln(writer, "Tags:<br>")
		fmt.Fprintln(writer,
			`<pre style="background-color: #eee; border: 1px solid #999; display: block; float: left;">`)
		json.WriteWithIndent(writer, "    ", img.Tags)
		fmt.Fprintln(writer, `</pre><p style="clear: both;">`)
	}
	fmt.Fprintf(writer, "File checksum: %x<br>\n", checksum)
	fmt.Fprintln(writer, "</body>")
}

func showAnnotation(writer io.Writer, annotation *image.Annotation,
	imageName string, linkName string, baseURL string) {
	if annotation == nil {
		return
	}
	var url string
	if annotation.URL != "" {
		url = annotation.URL
	} else {
		url = baseURL + "?" + imageName
	}
	fmt.Fprintf(writer, "<a href=\"%s\">%s</a><br>\n", url, linkName)
}
