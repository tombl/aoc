package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/pflag"
	"github.com/tombl/aoc"
)

func main() {
	theme := huh.ThemeBase()

	var args struct {
		day, year, part       int
		help, example, submit bool
		remainder             []string
	}

	now := time.Now()
	pflag.IntVarP(&args.day, "day", "d", now.Day(), "day")
	pflag.IntVarP(&args.year, "year", "y", now.Year(), "year")
	pflag.IntVarP(&args.part, "part", "p", 1, "part")
	pflag.BoolVarP(&args.help, "help", "h", false, "help")
	pflag.BoolVarP(&args.example, "example", "e", false, "get the example input (only used with a command)")
	pflag.BoolVarP(&args.submit, "submit", "s", false, "submit the output of the command as the answer")
	pflag.SetInterspersed(false)
	pflag.Parse()
	args.remainder = pflag.Args()

	if args.help ||
		(len(args.remainder) == 0 && (args.example || args.submit)) ||
		(args.example && args.submit) {
		pflag.Usage()
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("getting current working directory: %w", err))
	}

	aocDir, ok := findUp(cwd, ".aoc")
	if !ok {
		aocDir = filepath.Join(cwd, ".aoc")
		os.Mkdir(aocDir, 0755)
		os.WriteFile(filepath.Join(aocDir, ".gitignore"), []byte(`# This folder contains your session cookie,
# as well as your own non-redistributable puzzle inputs.
*`), 0644)
	}

	sessionFile := filepath.Join(aocDir, "session")
	sessionBytes, err := os.ReadFile(sessionFile)
	hasSession := err == nil
	session := strings.Trim(string(sessionBytes), "\n ")

	if !hasSession {
		if err := huh.
			NewInput().
			Title("Enter your session cookie for adventofcode.com").
			EchoMode(huh.EchoModePassword).
			Value(&session).
			Validate(func(s string) error {
				if ok := aoc.SessionCookieRegex.MatchString(s); !ok {
					return fmt.Errorf("invalid session cookie")
				}
				return nil
			}).
			WithTheme(theme).
			Run(); err != nil {
			if err == huh.ErrUserAborted {
				os.Exit(1)
			}
			panic(fmt.Errorf("requesting session cookie: %w", err))
		}
	}

	client, err := aoc.NewClient(session, filepath.Join(aocDir, "cache"))
	if err != nil {
		panic(fmt.Errorf("creating client: %w", err))
	}

	if !hasSession {
		if err := client.InvalidateUser(); err != nil {
			panic(fmt.Errorf("invalidating cached user: %w", err))
		}
		name, err := client.GetUser()
		if err != nil {
			panic(fmt.Errorf("checking authentication: %w", err))
		}
		fmt.Printf("Logged in as %s\n", name)

		_ = os.WriteFile(sessionFile, []byte(session), 0600)
	}

	if len(args.remainder) == 0 {
		day, err := client.GetDay(args.year, args.day)
		if err != nil {
			panic(fmt.Errorf("getting day: %w", err))
		}

		if isatty.IsTerminal(os.Stdout.Fd()) {
			part1, err := glamour.Render(day.Part1, "dark")
			if err != nil {
				panic(fmt.Errorf("rendering part 1: %w", err))
			}
			fmt.Println(part1)

			part2, err := glamour.Render(day.Part2, "dark")
			if err != nil {
				panic(fmt.Errorf("rendering part 2: %w", err))
			}
			fmt.Println(part2)
		} else {
			fmt.Println(day.Part1)
			fmt.Println("")
			fmt.Println(day.Part2)
		}
	} else {
		cmd := exec.Command(args.remainder[0], args.remainder[1:]...)

		cmd.Env = append(
			os.Environ(),
			fmt.Sprintf("AOC_PART=%d", args.part),
			fmt.Sprintf("AOC_DAY=%d", args.day),
			fmt.Sprintf("AOC_YEAR=%d", args.year),
		)

		var input io.ReadCloser
		if args.example {
			input, err = client.GetExample(args.year, args.day, args.part)
		} else {
			input, err = client.GetInput(args.year, args.day)
		}
		if err != nil {
			panic(fmt.Errorf("getting input: %w", err))
		}

		cmd.Stdin = input
		cmd.Stderr = os.Stderr

		var stdout strings.Builder
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)

		if err := cmd.Run(); err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				os.Exit(exit.ExitCode())
			} else {
				panic(fmt.Errorf("running command: %w", err))
			}
		}

		if args.submit {
			answer := strings.TrimSpace(stdout.String())
			shouldSubmit := false
			if err := huh.NewConfirm().
				Title("Submit answer?").
				Description(answer).
				Affirmative("Submit").
				Negative("Cancel").
				Value(&shouldSubmit).
				WithTheme(theme).
				Run(); err != nil {
				if err == huh.ErrUserAborted {
					os.Exit(1)
				}
				panic(fmt.Errorf("requesting confirmation: %w", err))
			}

			if shouldSubmit {
				result, err := client.SubmitAnswer(args.year, args.day, args.part, answer)
				if err != nil {
					panic(fmt.Errorf("submitting answer: %w", err))
				}
				fmt.Println(result)

				if !strings.Contains(result, "That's the right answer") {
					os.Exit(1)
				}
			} else {
				os.Exit(1)
			}
		}
	}
}
