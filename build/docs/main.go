package main

func main() {
	requireRepoRoot()

	if err := generateOpenAPI(); err != nil {
		panic(err)
	}

	generateReadme()
}
