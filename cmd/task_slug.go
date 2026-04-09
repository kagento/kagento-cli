package cmd

import (
	"fmt"
	"regexp"
	"strings"
)

const taskSlugMaxLength = 128

var taskSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validateTaskSlugValue(slug string) error {
	slug = strings.TrimSpace(slug)
	switch {
	case slug == "":
		return fmt.Errorf("slug is required")
	case len(slug) > taskSlugMaxLength:
		return fmt.Errorf("slug must be %d characters or fewer", taskSlugMaxLength)
	case !taskSlugPattern.MatchString(slug):
		return fmt.Errorf("slug must use lowercase letters, numbers, and single hyphens")
	default:
		return nil
	}
}
