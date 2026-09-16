package main

func main() {
	if err := generateOpenAPI(); err != nil {
		panic(err)
	}

	generateReadme()
}
