package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	spinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	dotStyle      = helpStyle.Copy().UnsetMargins()
	senderStyle = dotStyle.Copy()
	appStyle      = lipgloss.NewStyle().Margin(1, 2, 0, 2)
)

type message struct {
	content     string
	sender 		string
}

func (r message) String() string {
	if r.content == "" {
		return dotStyle.Render(strings.Repeat(".", 30))
	}
	if r.sender == "me" {
	return fmt.Sprintf("🐚  %s %s", r.content,
		senderStyle.Render(r.sender))
	}
	return fmt.Sprintf("🐠  %s %s", r.content,
		senderStyle.Render(r.sender))
}


type model struct {
	spinner  spinner.Model
	results  []message
	quitting bool
	textInput textinput.Model
	err       error
}

func newModel() model {
	ti := textinput.New()
	ti.Placeholder = "hello there"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	const numLastResults = 8
	s := spinner.New()
	s.Style = spinnerStyle
	return model{
		spinner: s,
		results: make([]message, numLastResults),
		textInput: ti,
		err:       nil,
	}
}


func (m model) Init() tea.Cmd {
	return spinner.Tick
	// return textinput.Blink
}


func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		var cmd tea.Cmd
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			m.results = append(m.results[1:], message{content: m.textInput.Value(), sender: "me"})
			broadcastHandler(m.textInput.Value())
			m.textInput.Reset()
			return m, cmd
		}
		
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	case message:
		m.results = append(m.results[1:], msg)
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m model) View() string {
	var s string

	if m.quitting {
		s += "That’s all for today!"
	} else {
		s += m.spinner.View() + " Online ..."
	}

	s += "\n\n"

	for _, res := range m.results {
		s += res.String() + "\n"
	}

	if !m.quitting {	
		s += helpStyle.Render("Type here")
		s += helpStyle.Render(m.textInput.View())
		s += helpStyle.Render("esc to exit")
	}

	if m.quitting {
		s += "\n"
	}

	return appStyle.Render(s)
}


func broadcastHandler(content string) {
	var wg sync.WaitGroup

	data, err := exec.Command("arp", "-a").Output()
	if err != nil {
		panic(err)
	}
	apichannel := make(chan string)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		ip := strings.Replace(fields[1], "(", "", -1)
		ip = strings.Replace(ip, ")", "", -1)
		url := "http://" + ip +":3998/receipt?content=" + content

		wg.Add(1)
		go MakeRequest(url, apichannel, &wg)

	}
	go func() {
			wg.Wait()
			close(apichannel)
		}()
}

func MakeRequest(url string, ch chan<- string, wg *sync.WaitGroup) {
	start := time.Now()
	resp, err := http.Get(url)

	if err == nil {
		secs := time.Since(start).Seconds()
		body, _ := ioutil.ReadAll(resp.Body)

		ch <- fmt.Sprintf("%.2f elapsed with response length: %d %s", secs, len(body), url)
	}
	defer wg.Done()
}


func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	
	done := make(chan struct{})
	prog := tea.NewProgram(newModel())


	go func() {
		http.HandleFunc("/receipt", func(w http.ResponseWriter, r *http.Request) {
		content := r.URL.Query().Get("content")
		// fmt.Println("content =>", content)
		prog.Send(message{content: content, sender: "them"})
	})

	fmt.Printf("Starting server at port 3998\n")
    if err := http.ListenAndServe(":3998", nil); err != nil {
        log.Fatal(err)
    }
		
	}()
	
	go func() {
		if err := prog.Start(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
		close(done)
	}()


	<-done
}

