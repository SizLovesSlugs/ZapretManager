package main

import (
	"fmt"

	"zapret-manager/internal/version"
)

func main() {
	fmt.Print(version.ExeName())
}
