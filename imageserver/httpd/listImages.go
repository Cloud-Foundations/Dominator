package httpd

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
)

func (s state) listImagesHandler(w http.ResponseWriter, req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	imageNames := s.imageDataBase.ListImages()
	verstr.Sort(imageNames)
	if req.URL.RawQuery == "output=text" {
		for _, name := range imageNames {
			fmt.Fprintln(writer, name)
		}
		return
	}
	fmt.Fprintln(writer, "<title>imageserver images</title>")
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<h3>")
	fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
	tw, _ := html.NewTableWriter(writer, true,
		"Name", "Data Size", "Data Inodes", "Computed Inodes", "Filter Lines",
		"Triggers", "Branch", "Commit")
	for _, name := range imageNames {
		if img := s.imageDataBase.GetImage(name); img != nil {
			writeImage(tw, name, img)
		}
	}
	tw.Close()
	fmt.Fprintln(writer, "</body>")
}

func writeImage(tw *html.TableWriter, name string, img *image.Image) {
	tw.WriteRow("", "",
		fmt.Sprintf("<a href=\"showImage?%s\">%s</a>", name, name),
		fmt.Sprintf("<a href=\"listImage?%s\">%s</a>",
			name, format.FormatBytes(img.FileSystem.TotalDataBytes)),
		fmt.Sprintf("<a href=\"listImage?%s\">%d</a>",
			name, img.FileSystem.NumRegularInodes),
		func() string {
			if num := img.FileSystem.NumComputedRegularInodes(); num < 1 {
				return "0"
			} else {
				return fmt.Sprintf("<a href=\"listComputedInodes?%s\">%d</a>",
					name, num)
			}
		}(),
		func() string {
			if img.Filter == nil {
				return "(sparse filter)"
			} else if len(img.Filter.FilterLines) < 1 {
				return "0"
			} else {
				return fmt.Sprintf("<a href=\"listFilter?%s\">%d</a>",
					name, len(img.Filter.FilterLines))
			}
		}(),
		func() string {
			if img.Triggers == nil || len(img.Triggers.Triggers) < 1 {
				return "0"
			} else {
				return fmt.Sprintf("<a href=\"listTriggers?%s\">%d</a>",
					name, len(img.Triggers.Triggers))
			}
		}(),
		img.BuildBranch,
		img.BuildCommitId,
	)
}
