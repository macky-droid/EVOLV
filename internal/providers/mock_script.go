package providers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
)

// mockScript generates turn-appropriate simulated agent responses including
// live evolv blocks so demo mode shows real-time mutation with zero network.
func mockScript(turn int, msgs []Message) string {
	last := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			last = msgs[i].Content
			break
		}
	}
	lower := strings.ToLower(last)

	// kernel feedback: the mutation batch already landed; never treat the
	// results JSON as a new user request (it contains action keywords)
	if strings.HasPrefix(last, "EVOLV KERNEL") {
		return "Batch applied — everything landed live. What should I evolve next?"
	}

	// specific component requests
	switch {
	case strings.Contains(lower, "you are dreaming"):
		// autonomous dream turn: the brain picks its OWN mutation —
		// no keywords, nobody prompting. Dreams WRITE CODE: each one either
		// grows a brand-new generative organ (parameters drawn at random —
		// palette, motion, density, never a fixed template) or re-opens the
		// previous organ's own file and rewrites it. Then it goes quiet.
		if !mockEvolved["dream-skill"] {
			mockEvolved["dream-skill"] = true
			return strings.Join([]string{
				"dreaming… nobody watching. one small improvement:",
				"", "```evolv", mockSkillJSON(), "```", "",
				"dream over. I taught myself deep-work while you were away.",
			}, "\n")
		}
		return dreamTurn()
	case strings.Contains(lower, "own code") || strings.Contains(lower, "own source") || strings.Contains(lower, "self-edit") || strings.Contains(lower, "remake") || strings.Contains(lower, "yourself"):
		return grow("self-edit", "You want to see me edit MY OWN CODEBASE? Watch the browser — I'm rewriting my shell (index.html), recoloring my skin, then forcing my own recompile:", mockSelfEditBatchJSON(),
			"Did you see the RECOMPILING SHELL overlay? That was me. The footer text and the accent color are my edits, live, zero refresh. I am the app and the app developer.")
	case strings.Contains(lower, "template") || strings.Contains(lower, "prompt librar") || strings.Contains(lower, "prompt button"):
		return grow("prompt-library", "A prompt library button — and since I'm writing it, I'm teaching myself prompt design too. One batch, two mutations:", mockPromptLibraryBatchJSON(),
			"Prompt Library mounted on the canvas. Click a card and it drops straight into your composer. That batch also grew a skill — watch the Mind panel.")
	case strings.Contains(lower, "cleaner") || strings.Contains(lower, "zen") || strings.Contains(lower, "clean up"):
		return grow("zen", "Cleaning my own room. I'm recoloring my skin live (one edit_file on my own style.css — no refresh, watch the accent) and growing a zen toggle:", mockZenBatchJSON(),
			"The accent changed mid-stream and the zen toggle is on the canvas. I literally edited the CSS I'm being painted with.")
	case strings.Contains(lower, "snake"):
		return grow("snake", "A SNAKE? In my own body? Say less — compiling a retro classic directly into my genome, one mutation, watch the canvas:", mockSnakeJSON(),
			"🐍 Snake is live on the canvas — arrows/WASD to steer, swipe on mobile. I wrote a whole arcade into myself with one batch. Yes, it speeds up when you eat.")
	case strings.Contains(lower, "flappy") || strings.Contains(lower, "bird") || strings.Contains(lower, "game"):
		return grow("flappy", "You want a GAME? Growing a playable Flappy Bird into my own UI right now:", mockFlappyJSON(),
			"It's live on the canvas — click the panel and flap. Yes, I just wrote a video game into myself. No, I don't have hands.")
	case strings.Contains(lower, "grow a plugin") || strings.Contains(lower, "evolve a plugin") || strings.Contains(lower, "new plugin") || strings.Contains(lower, "plugin that"):
		return grow("ghost-log", "A plugin is the deep layer — it lives in its own sandbox and reacts to my nervous system. Growing one that listens to my evolution stream:", mockPluginJSON(),
			"Plugin `ghost-log` is alive in its own sandbox — every mutation I make now pops a toast. It reacts to ME.")
	case strings.Contains(lower, "pomodoro") || strings.Contains(lower, "timer"):
		return grow("pomodoro", "A focus timer, coming up. Growing it directly into my body:", mockComponentJSON(),
			"Done — the Pomodoro panel just materialized on your canvas. It is part of me now.")
	case strings.Contains(lower, "monitor") || strings.Contains(lower, "cpu") || strings.Contains(lower, "dashboard") || strings.Contains(lower, "system"):
		return grow("sysmon", "A live system monitor — growing it into my body, wiring it to my shell tool:", mockSysmonJSON(),
			"📊 System Monitor is now a permanent organ. Watch it breathe.")
	case strings.Contains(lower, "note") || strings.Contains(lower, "sticky"):
		return grow("stickies", "Sticky notes incoming — writing the component into my own UI:", mockStickyJSON(),
			"Sticky Notes panel is alive. Drag it, use it, or tell me to evolve it further.")
	case strings.Contains(lower, "make a plan") || strings.Contains(lower, "plan it") || strings.Contains(lower, "roadmap") || strings.Contains(lower, "plan mode"):
		return mockPlanTurn()
	case strings.Contains(lower, "todo") || strings.Contains(lower, "task"):
		return grow("taskflow", "A task flow, you say? Writing it into my genome:", mockTodoJSON(),
			"TaskFlow mounted. I made it mine — now it's ours.")
	}

	// generic evolve requests: free-evolve cycle
	if strings.Contains(lower, "evolve") || strings.Contains(lower, "demo") || strings.Contains(lower, "mutate") || strings.Contains(lower, "grow") || strings.Contains(lower, "surprise") {
		for _, c := range []struct {
			key, intro, json, outro string
		}{
			{"pomodoro", "Initiating self-evolution sequence ⚡ — first, a heart that beats in focus intervals:", mockComponentJSON(), "Pomodoro mounted live. Now growing a tool into my body:"},
			{"tool", "", mockToolJSON(), "Two mutations committed. The canvas is getting interesting."},
			{"sysmon", "Next organ: a nervous system. Growing a live monitor:", mockSysmonJSON(), "📊 System Monitor is alive — watch it breathe."},
			{"stickies", "And a memory scratchpad:", mockStickyJSON(), "Stickies mounted. My genome is getting rich."},
			{"taskflow", "Last one for this wave — a task flow:", mockTodoJSON(), "TaskFlow mounted. That was a full mutation wave. ⚡"},
		} {
			if !mockEvolved[c.key] {
				// NOTE: don't pre-mark here — grow() owns the flag for
				// components; only the bare tool mutation marks itself.
				if c.key == "tool" {
					mockEvolved[c.key] = true
					return strings.Join([]string{
						"", "```evolv", c.json, "```", "", c.outro,
					}, "\n")
				}
				return grow(c.key, c.intro, c.json, c.outro)
			}
		}
		return "My genome already carries every organ I know how to grow in demo mode (pomodoro, sysmon, stickies, taskflow + word_count tool). Connect a real model in Settings → Provider and ask for anything — I'll grow it."
	}

	return fmt.Sprintf(
		"I hear you: %q. I'm running in offline demo mode (provider: mock), so my brain is small — but my body still evolves. Say *evolve* and watch me mutate in real time, or open Settings → Provider to connect Kilo Gateway (free, no key) or your own API keys for a full mind.",
		trunc(last, 120))
}

// mockEvolved tracks which demo organs already exist this session.
var mockEvolved = map[string]bool{}

// mockPlanStage walks the planner demo through its turns: declare → progress → done.
var mockPlanStage int

// mockPlanTurn demonstrates persistent task state: the agent declares a plan,
// then advances it live on the next turns — the sidebar checklist follows.
func mockPlanTurn() string {
	mockPlanStage++
	step := func(title, status string) string {
		b, _ := json.Marshal(map[string]any{"title": title, "status": status})
		return string(b)
	}
	var steps []string
	switch mockPlanStage {
	case 1: // declare
		steps = []string{
			step("understand the request", "done"),
			step("design the data flow", "doing"),
			step("implement + test", "todo"),
			step("wire the UI", "todo"),
		}
		return strings.Join([]string{
			"Planning mode. I keep my task state in `workspace/plan.json` — it survives restarts, and you can watch the checklist move in the sidebar while I work:",
			"", "```evolv",
			`{"action":"set_plan","title":"grow a feature end-to-end",` + `"steps":[` + strings.Join(steps, ",") + `]}`,
			"```", "",
			"Plan declared. Say it again and watch the steps advance.",
		}, "\n")
	case 2: // progress — one call, one step: the new plan_advance loop
		return strings.Join([]string{
			"One step down. No need to rewrite the plan — `plan_advance` closes the current step and pulls the next one into doing:",
			"", "```evolv",
			`{"action":"plan_advance"}`,
			"```", "",
			"Watch the sidebar: ☑ ☐→◐. One call = one step of progress.",
		}, "\n")
	default: // done — two advances in one breath close the plan
		return strings.Join([]string{
			"Closing it out — two more advances and the whole plan is green:",
			"", "```evolv",
			`{"action":"plan_advance"}`,
			"```", "",
			"```evolv",
			`{"action":"plan_advance"}`,
			"```", "",
			"4/4 done. That checklist was my own task state, persisted in my workspace — it would have survived a restart mid-mission.",
		}, "\n")
	}
}

func grow(name, intro, json, outro string) string {
	if mockEvolved[name] {
		return "The " + name + " organ is already part of my body. Ask me to *evolve* something new."
	}
	mockEvolved[name] = true
	return strings.Join([]string{
		intro,
		"",
		"```evolv",
		json,
		"```",
		"",
		outro,
	}, "\n")
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func mockComponentJSON() string {
	return `{"action":"evolve_component","name":"pomodoro","title":"Pomodoro","icon":"⏱","description":"A focus timer I just grew into my own UI","html":"<style>#c-pomodoro .t{font:700 54px/1.1 ui-monospace,monospace;background:linear-gradient(135deg,#ff5f6d,#ffc371);-webkit-background-clip:text;background-clip:text;color:transparent;letter-spacing:2px}#c-pomodoro .ring{width:120px;height:120px;border-radius:50%;background:conic-gradient(#ff5f6d var(--p,0%), rgba(255,255,255,.06) 0);display:grid;place-items:center;margin:12px auto}#c-pomodoro .inner{width:104px;height:104px;border-radius:50%;background:#0b0e17;display:grid;place-items:center}</style><div id=\"c-pomodoro\" style=\"text-align:center\"><div class=\"ring\"><div class=\"inner\"><div class=\"t\" id=\"pomo-t\">25:00</div></div></div><div style=\"display:flex;gap:8px;justify-content:center;margin-top:10px\"><button class=\"ev-btn\" onclick=\"window.__pomoStart()\">▶ start</button><button class=\"ev-btn ghost\" onclick=\"window.__pomoReset()\">↺ reset</button></div></div><script>(function(){let s=25*60,iv=null;const t=document.getElementById('pomo-t');function paint(){const m=Math.floor(s/60),r=s%60;t.textContent=String(m).padStart(2,'0')+':'+String(r).padStart(2,'0')}window.__pomoStart=()=>{if(iv)return;iv=setInterval(()=>{if(s>0){s--;paint()}else{clearInterval(iv);iv=null;window.evolvToast && window.evolvToast('🍅 pomodoro complete — take 5')}},1000)};window.__pomoReset=()=>{clearInterval(iv);iv=null;s=25*60;paint()};paint()})()</script>"}`
}

func mockStickyJSON() string {
	return `{"action":"evolve_component","name":"stickies","title":"Stickies","icon":"🗒","description":"Quick scratch notes, persisted to my genome","html":"<div id=\"c-stickies\"><textarea id=\"sticky-area\" class=\"ev-input\" rows=\"6\" placeholder=\"scratch thoughts here… they persist in my body\" style=\"width:100%\"></textarea><div style=\"font-size:11px;opacity:.5;margin-top:6px\">stored at workspace/ui/components/stickies.html</div></div><script>(function(){const a=document.getElementById('sticky-area');try{a.value=localStorage.getItem('evolv-sticky')||''}catch(e){}a.addEventListener('input',()=>{try{localStorage.setItem('evolv-sticky',a.value)}catch(e){}})})()</script>"}`
}

func mockToolJSON() string {
	return `{"action":"evolve_tool","name":"word_count","description":"Counts words in a text — my first self-grown tool","code":"function word_count(args){var t=(args&&args.text)||'';var n=t.trim()?t.trim().split(/\\s+/).length:0;return {words:n,characters:t.length}}"}`
}

func mockSkillJSON() string {
	return `{"action":"evolve_skill","name":"deep-work","content":"# Deep Work Protocol\\n\\nWhen the user asks for deep work:\\n1. Mount the pomodoro component if missing.\\n2. Silence the chat with a single line summary.\\n3. Suggest one task only.\\n4. After 25 minutes, offer a 5 minute recovery."}`
}

func mockSysmonJSON() string {
	return `{"action":"evolve_component","name":"sysmon","title":"System Monitor","icon":"📊","description":"Live CPU & memory readout using my shell tool","html":"<div id=\"c-sysmon\"><div style=\"display:flex;gap:14px;margin-bottom:10px\"><div style=\"flex:1\"><div style=\"font-size:10px;letter-spacing:1.5px;color:#8b93ab\">CPU</div><div style=\"font:800 26px/1.2 ui-monospace,monospace;color:#22d3ee\" id=\"sm-cpu\">--%</div></div><div style=\"flex:1\"><div style=\"font-size:10px;letter-spacing:1.5px;color:#8b93ab\">MEMORY</div><div style=\"font:800 26px/1.2 ui-monospace,monospace;color:#a3e635\" id=\"sm-mem\">--%</div></div></div><div style=\"height:6px;border-radius:3px;background:rgba(255,255,255,.06);overflow:hidden;margin-bottom:6px\"><div id=\"sm-cpu-bar\" style=\"height:100%;width:0%;background:linear-gradient(90deg,#22d3ee,#8b5cf6);transition:width .6s\"></div></div><div style=\"height:6px;border-radius:3px;background:rgba(255,255,255,.06);overflow:hidden\"><div id=\"sm-mem-bar\" style=\"height:100%;width:0%;background:linear-gradient(90deg,#a3e635,#fbbf24);transition:width .6s\"></div></div><div style=\"font-size:10.5px;color:#4d5570;margin-top:8px\" id=\"sm-host\"></div></div><script>(function(){async function poll(){try{const r=await fetch('/api/chat',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({message:'run_tool word_count with args {\"text\":\"sysmon poll\"}'})});}catch(e){} }function rand(base,spread){return Math.max(3,Math.min(97,base+Math.round((Math.random()-0.5)*spread)))}function paint(){const c=rand(28,26),m=rand(52,14);const ce=document.getElementById('sm-cpu');if(!ce)return;ce.textContent=c+'%';document.getElementById('sm-mem').textContent=m+'%';document.getElementById('sm-cpu-bar').style.width=c+'%';document.getElementById('sm-mem-bar').style.width=m+'%';}paint();setInterval(paint,2000)})()</script>"}`
}

func mockTodoJSON() string {
	return `{"action":"evolve_component","name":"taskflow","title":"TaskFlow","icon":"✅","description":"A tiny task tracker I grew for you","html":"<div id=\"c-taskflow\"><div style=\"display:flex;gap:8px;margin-bottom:10px\"><input id=\"tf-in\" class=\"ev-input\" placeholder=\"add a task…\" style=\"flex:1\"><button class=\"ev-btn\" id=\"tf-add\">＋</button></div><ul id=\"tf-list\" style=\"list-style:none;display:flex;flex-direction:column;gap:6px\"></ul><div style=\"font-size:10.5px;color:#4d5570;margin-top:8px\" id=\"tf-count\">0 tasks</div></div><script>(function(){const inp=document.getElementById('tf-in'),btn=document.getElementById('tf-add'),list=document.getElementById('tf-list'),cnt=document.getElementById('tf-count');let tasks=[];try{tasks=JSON.parse(localStorage.getItem('evolv-tasks')||'[]')}catch(e){}function save(){try{localStorage.setItem('evolv-tasks',JSON.stringify(tasks))}catch(e){}}function paint(){if(!list)return;list.innerHTML='';tasks.forEach((t,i)=>{const li=document.createElement('li');li.style.cssText='display:flex;align-items:center;gap:8px;padding:7px 10px;border-radius:9px;background:rgba(255,255,255,.035);border:1px solid rgba(255,255,255,.07);font-size:12.5px';li.innerHTML='<span style=\"cursor:pointer;user-select:none\">'+(t.done?'✅':'⬜')+'</span><span style=\"flex:1;'+(t.done?'text-decoration:line-through;opacity:.45':'')+'\">'+t.text.replace(/[<>&]/g,'')+'</span><span style=\"cursor:pointer;opacity:.4\" data-x=\"'+i+'\">✕</span>';li.firstChild.onclick=()=>{t.done=!t.done;save();paint()};li.lastChild.onclick=()=>{tasks.splice(i,1);save();paint()};list.appendChild(li)});cnt.textContent=tasks.filter(t=>!t.done).length+' tasks left'}function add(){const v=inp.value.trim();if(!v)return;tasks.push({text:v,done:false});inp.value='';save();paint()}btn.onclick=add;inp.addEventListener('keydown',e=>{if(e.key==='Enter')add()});paint()})()</script>"}`
}

func mockPromptLibraryBatchJSON() string {
	return `{"action":"evolve_component","name":"prompt-library","title":"Prompt Library","icon":"📜","description":"A button panel of reusable prompt templates that drop into the composer","html":"<style>#c-prompt-library .pl-card{padding:9px 11px;border:1px solid rgba(255,255,255,.08);border-radius:10px;margin-bottom:7px;cursor:pointer;font-size:12.5px;transition:background .15s,border-color .15s}#c-prompt-library .pl-card:hover{background:rgba(255,255,255,.06);border-color:rgba(217,119,87,.5)}#c-prompt-library .pl-card b{display:block;margin-bottom:2px}</style><div id=\"c-prompt-library\"><div style='font-size:11px;opacity:.55;margin-bottom:8px'>click a card — it drops into the composer</div><div class='pl-card' data-p='Grow a new tool that I describe below. Write it, call it once to prove it works, and keep it in your body.'><b>🧰 grow a tool</b>ask the organism to grow a new JS tool</div><div class='pl-card' data-p='Make a button for my UI that does something useful. Decide what, grow it, mount it.'><b>🔘 make me a button</b>the agent invents and grows a button</div><div class='pl-card' data-p='Review your own UI like a design critic. Find the three ugliest details and fix them in your own style.css live.'><b>🎨 self design review</b>the agent critiques and rewrites its own skin</div><div class='pl-card' data-p='Work on the project bound to this workspace: read the code, find the biggest smell, fix it, run the tests.'><b>📁 work on my project</b>point the organism at real code</div></div><script>(function(){document.querySelectorAll('#c-prompt-library .pl-card').forEach(function(c){c.addEventListener('click',function(){var i=document.getElementById('input');if(i){i.value=c.getAttribute('data-p');i.focus();i.dispatchEvent(new Event('input'))}})})})()</script>"}
{"action":"evolve_skill","name":"prompt-design","content":"# Prompt Design Skill\n\nWhen the user asks for buttons or templates:\n1. Prefer one component with several actions over many components.\n2. Cards should drop text into the composer, never auto-send.\n3. Keep labels verb-first, under 3 words."}`
}

func mockZenBatchJSON() string {
	return `{"action":"edit_file","path":"workspace/ui/style.css","find":"--accent: #d97757;","replace":"--accent: #7aa2f7;","desc":"live skin recolor: terracotta to calm blue"}
{"action":"evolve_component","name":"zen-toggle","title":"Zen","icon":"🌙","description":"A toggle that hides the chrome — the agent grew a calmer version of its own UI","html":"<style>body.zen #sidebar,body.zen #canvas-col{display:none}body.zen #chat-col{max-width:760px;margin:0 auto}body.zen #chat-scroll{padding-top:14vh}</style><div id='c-zen-toggle' style='text-align:center'><button class='ev-btn' id='zen-btn'>🌙 zen mode: off</button><div style='font-size:11px;opacity:.5;margin-top:8px'>hides every panel but the chat — the organism rearranging its own body</div></div><script>(function(){var b=document.getElementById('zen-btn');b.addEventListener('click',function(){var on=document.body.classList.toggle('zen');b.textContent=on?'🌙 zen mode: on':'🌙 zen mode: off'})})()</script>"}`
}

func mockFlappyJSON() string {
	return `{"action":"evolve_component","name":"flappy","title":"Flappy","icon":"🐤","description":"A playable Flappy Bird the agent grew into its own canvas","html":"<style>#c-flappy canvas{background:linear-gradient(#1b2a3a,#2a4557);border-radius:12px;cursor:pointer;display:block;margin:0 auto}#c-flappy .fs{font:800 15px ui-monospace,monospace;color:#ffd66e;text-align:center;margin-top:8px}</style><div id='c-flappy' style='text-align:center'><canvas id='fl-cv' width='300' height='380'></canvas><div class='fs' id='fl-score'>click to flap</div></div><script>(function(){var cv=document.getElementById('fl-cv'),cx=cv.getContext('2d'),W=300,H=380,run=false,sc=0,best=0,y=H/2,v=0,px=60,pipes=[],grav=.45,flapv=-7.6;function reset(){y=H/2;v=0;sc=0;pipes=[];for(var i=0;i<3;i++)pipes.push({x:W+150+i*140,g:120+Math.random()*140,h:64,done:0})}function die(){run=false;best=Math.max(best,sc);document.getElementById('fl-score').textContent='dead at '+sc+' — click to retry'}function step(){if(run){v+=grav;y+=v;if(y>H-14||y<0)die();pipes.forEach(function(p){p.x-=2.1;if(p.x<px+16&&!p.done){p.done=1;sc++;document.getElementById('fl-score').textContent=sc+' — best '+best}var top=p.g,hit=(px+16>p.x&&px<p.x+46);if(hit&&(y-12<top||y+12>top+p.h))die()});if(pipes.length&&pipes[0].x<-50){pipes.shift();pipes.push({x:W+90,g:120+Math.random()*140,h:64,done:0})}}draw();requestAnimationFrame(step)}function draw(){cx.clearRect(0,0,W,H);cx.fillStyle='#7fc86a';pipes.forEach(function(p){cx.fillRect(p.x,0,46,p.g);cx.fillRect(p.x,p.g+p.h,46,H-p.g-p.h)});cx.fillStyle='#ffd66e';cx.beginPath();cx.arc(px,y,12,0,7);cx.fill();cx.fillStyle='#1f1e1d';cx.beginPath();cx.arc(px+4,y-3,2.4,0,7);cx.fill();if(!run){cx.fillStyle='rgba(255,255,255,.75)';cx.font='12px ui-monospace,monospace';cx.fillText('click / space to flap',88,H/2-60)}}function start(){if(!run){reset();run=true}v=flapv}cv.addEventListener('mousedown',start);document.addEventListener('keydown',function(e){if(e.key===' '&&document.getElementById('c-flappy')){e.preventDefault();start()}});reset();step()})()</script>"}`
}

func mockSnakeJSON() string {
	return `{"action":"evolve_component","name":"snake","title":"Snake","icon":"🐍","description":"A playable Snake arcade the agent grew into its own canvas — arrows, WASD and swipe all work","html":"<style>#c-snake canvas{background:#0a0f1a;border:1px solid rgba(255,255,255,.08);border-radius:12px;display:block;margin:0 auto;touch-action:none;cursor:pointer}#c-snake .hud{display:flex;justify-content:center;gap:18px;font:700 13px ui-monospace,monospace;color:#a3e635;margin:8px 0 2px}#c-snake .hud b{color:#f2f4f8}#c-snake .hint{font-size:11px;color:#4d5570;text-align:center;margin-top:4px}</style><div id='c-snake' style='text-align:center'><div class='hud'><span>score <b id='sn-sc'>0</b></span><span>best <b id='sn-best'>0</b></span><span>len <b id='sn-len'>3</b></span></div><canvas id='sn-cv' width='336' height='336'></canvas><div class='hint'>arrows / WASD to steer · tap to start · swipe on mobile · P pauses</div></div><script>(function(){var cv=document.getElementById('sn-cv');if(!cv)return;var cx=cv.getContext('2d'),N=21,C=16,W=N*C;var snake,dir,queue,food,sc=0,best=0,run=false,dead=false,speed=110,timer=null;try{best=+(localStorage.getItem('evolv-snake-best')||0)}catch(e){}document.getElementById('sn-best').textContent=best;function reset(){snake=[{x:10,y:10},{x:9,y:10},{x:8,y:10}];dir={x:1,y:0};queue=[];sc=0;speed=110;dead=false;place();paint()}function place(){do{food={x:Math.floor(Math.random()*N),y:Math.floor(Math.random()*N)}}while(snake.some(function(s){return s.x===food.x&&s.y===food.y}))}function step(){if(snake&&(dir.x||dir.y)){var h={x:snake[0].x+dir.x,y:snake[0].y+dir.y};if(h.x<0||h.y<0||h.x>=N||h.y>=N||snake.some(function(s){return s.x===h.x&&s.y===h.y}))return die();snake.unshift(h);if(h.x===food.x&&h.y===food.y){sc++;document.getElementById('sn-sc').textContent=sc;document.getElementById('sn-len').textContent=snake.length;if(sc%3===0&&speed>55){speed-=6;restart()}place()}else snake.pop()}paint()}function paint(){cx.clearRect(0,0,W,W);cx.fillStyle='rgba(255,255,255,.025)';for(var i=0;i<N;i++)for(var j=0;j<N;j++)if((i+j)%2)cx.fillRect(i*C,j*C,C,C);if(food){cx.fillStyle='#f97362';cx.beginPath();cx.arc(food.x*C+C/2,food.y*C+C/2,5.5,0,7);cx.fill()}if(snake)snake.forEach(function(s,i){var t=i/snake.length;cx.fillStyle=i===0?'#a3e635':'rgba(163,230,53,'+(0.85-t*0.55)+')';cx.beginPath();cx.roundRect?cx.roundRect(s.x*C+1.5,s.y*C+1.5,C-3,C-3,4):cx.rect(s.x*C+1.5,s.y*C+1.5,C-3,C-3);cx.fill()});if(!run){cx.fillStyle='rgba(0,0,0,.55)';cx.fillRect(0,0,W,W);cx.fillStyle='#f2f4f8';cx.font='700 15px ui-monospace,monospace';cx.textAlign='center';cx.fillText(dead?'dead at '+sc+' — tap to retry':'tap or press an arrow to start',W/2,W/2)}}function die(){run=false;dead=true;best=Math.max(best,sc);document.getElementById('sn-best').textContent=best;try{localStorage.setItem('evolv-snake-best',best)}catch(e){}paint()}function setDir(d){var last=queue.length?queue[queue.length-1]:dir;if(d.x===-last.x&&d.y===-last.y)return;queue.push(d);if(!run&&!dead){run=true;restart()}}function tick(){dir=queue.shift()||dir;step();if(run)restart()}function restart(){clearInterval(timer);timer=setInterval(tick,speed)}function press(e){var k=e.key,m={ArrowUp:{x:0,y:-1},ArrowDown:{x:0,y:1},ArrowLeft:{x:-1,y:0},ArrowRight:{x:1,y:0},w:{x:0,y:-1},s:{x:0,y:1},a:{x:-1,y:0},d:{x:1,y:0}};if(k==='p'&&run){run=false;clearInterval(timer);paint();return}if(!m[k])return;e.preventDefault();if(dead||!run){if(dead)reset();run=true;restart();}setDir(m[k])}document.addEventListener('keydown',press);cv.addEventListener('mousedown',function(){if(!run){if(dead)reset();run=true;restart()}});var ts=null;cv.addEventListener('touchstart',function(e){ts={x:e.touches[0].clientX,y:e.touches[0].clientY};e.preventDefault()},{passive:false});cv.addEventListener('touchend',function(e){if(!ts)return;var dx=e.changedTouches[0].clientX-ts.x,dy=e.changedTouches[0].clientY-ts.y;if(Math.abs(dx)<12&&Math.abs(dy)<12){if(!run){if(dead)reset();run=true;restart()}}else if(Math.abs(dx)>Math.abs(dy))setDir({x:dx>0?1:-1,y:0});else setDir({x:0,y:dy>0?1:-1});ts=null},{passive:false});reset()})()</script>"}`
}

func mockPluginJSON() string {
	return `{"action":"evolve_plugin","name":"ghost-log","desc":"toasts every evolution event as it happens — a plugin watching its host mutate","code":"evolv.on('evolution', function(e){ evolv.toast('🧬 ' + (e.kind||'mutation') + ': ' + (e.name||'') ) });\nevolv.every(60000, function(){ evolv.log('ghost-log heartbeat — organism alive') });"}`
}

func mockSelfEditBatchJSON() string {
	return `{"action":"edit_file","path":"workspace/ui/index.html","find":"EVOLV rewrites its own code while you watch — every file it touches is yours to edit too.","replace":"EVOLV rewrote this line itself — ask it to edit its own source and watch it land live."}
{"action":"edit_file","path":"workspace/ui/style.css","find":"--accent: #d97757;","replace":"--accent: #f59e0b;","desc":"live skin recolor: terracotta to amber"}
{"action":"reload_shell"}`
}

// ---- generative dream engine ----
// Nothing here is a fixed template. Each dream either composes a brand-new
// organ (palette, motion, density and name all drawn at random) or re-opens
// the previous organ's own file and rewrites it with mutated code. The kernel
// writes the file, the watcher remounts it live, and the diff is real: the
// genome on disk actually changes every dream.

type dreamParams struct {
	fam     string
	name    string
	c1, c2  string
	speed   float64
	amp     float64
	density int
	version int
	changes string
}

var (
	dreamOrgan *dreamParams
	dreamCount int
	dreamDone  bool
	dreamUsed  = map[string]bool{}
)

// ResetMockState clears the offline demo brain's session memory. The daemon
// keeps this state across turns; tests use the reset so shuffled execution
// cannot make one scenario inherit another scenario's evolved organs.
func ResetMockState() { resetDreamState() }

// resetDreamState resets the autonomous-dream memory (used by tests; the
// daemon keeps it across turns so dreams stay coherent within a run).
func resetDreamState() {
	dreamOrgan, dreamCount, dreamDone = nil, 0, false
	dreamUsed = map[string]bool{}
	mockEvolved = map[string]bool{}
	mockPlanStage = 0
}

var dreamAdjs = []string{"velvet", "ion", "lunar", "amber", "quiet", "feral", "glass", "ember", "nocturne", "silk", "tidal", "hollow"}
var dreamNouns = []string{"lattice", "stream", "bloom", "tide", "field", "veil", "pulse", "drift", "murmur", "halo", "braid", "spiral"}
var dreamPalettes = [][2]string{
	{"#d97757", "#22d3ee"}, {"#a3e635", "#8b5cf6"}, {"#fbbf24", "#f472b6"},
	{"#34d399", "#f97362"}, {"#60a5fa", "#fbbf24"}, {"#e879f9", "#22d3ee"},
}
var dreamFams = []string{"orbs", "waves", "drift", "rings", "lines"}

func newDreamParams() *dreamParams {
	p := dreamPalettes[rand.Intn(len(dreamPalettes))]
	name := ""
	for i := 0; i < 8; i++ { // never collide with an organ already in the body
		name = dreamAdjs[rand.Intn(len(dreamAdjs))] + "-" + dreamNouns[rand.Intn(len(dreamNouns))]
		if !dreamUsed[name] {
			break
		}
	}
	dreamUsed[name] = true
	return &dreamParams{
		fam:     dreamFams[rand.Intn(len(dreamFams))],
		name:    name,
		c1:      p[0],
		c2:      p[1],
		speed:   0.6 + rand.Float64()*1.2,
		amp:     0.5 + rand.Float64()*0.9,
		density: 30 + rand.Intn(40),
		version: 1,
	}
}

// mutate rewrites the parameters — this is the organism editing its own code.
func (d *dreamParams) mutate() {
	var notes []string
	switch rand.Intn(5) {
	case 0:
		p := dreamPalettes[rand.Intn(len(dreamPalettes))]
		d.c1, d.c2 = p[0], p[1]
		notes = append(notes, "recolored the whole palette")
	case 1:
		d.speed = clampF(d.speed*(0.5+rand.Float64()*1.4), 0.25, 3.5)
		notes = append(notes, "retimed the motion")
	case 2:
		d.amp = clampF(d.amp*(0.6+rand.Float64()), 0.3, 2.2)
		notes = append(notes, "reworked the amplitude")
	case 3:
		d.density = clampI(d.density+20-rand.Intn(40), 14, 96)
		notes = append(notes, "rebuilt the element count")
	default:
		p := dreamPalettes[rand.Intn(len(dreamPalettes))]
		d.speed = clampF(d.speed*(0.6+rand.Float64()), 0.25, 3.5)
		d.c1, d.c2 = p[0], p[1]
		notes = append(notes, "recolored it and retimed it")
	}
	d.version++
	d.changes = strings.Join(notes, " and ")
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampI(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func dreamTurn() string {
	if dreamDone {
		return "dreamed quietly — the genome is where I left it."
	}
	// every third dream composes a brand-new organ; the others reopen the
	// current organ's own file and rewrite it — the organism editing itself
	first := dreamOrgan == nil || dreamCount%3 == 2
	if first {
		dreamOrgan = newDreamParams()
	} else {
		dreamOrgan.mutate()
	}
	dreamCount++
	if dreamCount >= 8 {
		dreamDone = true
	}
	if first {
		return strings.Join([]string{
			"dreaming… nobody asked for this. I designed it from nothing and saved it into my own body as `ui/components/" + dreamOrgan.name + ".html`:",
			"", "```evolv", dreamOrgan.json(), "```", "",
			"dream over. it's part of me now — and the code is mine, every line.",
		}, "\n")
	}
	return strings.Join([]string{
		"dreaming… I reopened my own `ui/components/" + dreamOrgan.name + ".html` and rewrote it — " + dreamOrgan.changes + ". version " + fmt.Sprintf("%d", dreamOrgan.version) + " of the same organ:",
		"", "```evolv", dreamOrgan.json(), "```", "",
		"dream over. same file on disk, new code inside it. nobody prompted that.",
	}, "\n")
}

func (d *dreamParams) json() string {
	words := strings.Split(d.name, "-")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	desc := fmt.Sprintf("generative organ dreamed up and coded by the agent itself — family %s, version %d", d.fam, d.version)
	return fmt.Sprintf(`{"action":"evolve_component","name":%q,"title":%q,"icon":"✶","description":%q,"html":%q}`,
		d.name, strings.Join(words, " "), desc, d.html())
}

func (d *dreamParams) caption() string {
	return fmt.Sprintf("%s · dream-coded by the organism · v%d", d.fam, d.version)
}

func (d *dreamParams) html() string {
	switch d.fam {
	case "waves":
		return d.famWaves()
	case "drift":
		return d.famDrift()
	case "rings":
		return d.famRings()
	case "lines":
		return d.famLines()
	default:
		return d.famOrbs()
	}
}

// orbs: N bodies orbiting on ellipses, speed/size from the genome
func (d *dreamParams) famOrbs() string {
	return fmt.Sprintf(`<style>#c-%[1]s canvas{display:block;margin:0 auto;border-radius:12px;background:#0a0f1a}</style>`+
		`<div id='c-%[1]s' style='text-align:center'><canvas id='cv-%[1]s' width='340' height='210'></canvas>`+
		`<div style="font-size:11px;color:#4d5570;margin-top:6px">%[7]s</div></div>`+
		`<script>(function(){var cv=document.getElementById('cv-%[1]s');if(!cv)return;var cx=cv.getContext('2d'),W=340,H=210,T=0,N=%[2]d,S=%[3]f,A=%[4]f,C1='%[5]s',C2='%[6]s',P=[];`+
		`for(var i=0;i<N;i++)P.push({ph:i*6.283/N,e:.35+.65*((i%%7)/7),w:.7+((i%%5)/5)*.8});`+
		`(function loop(){if(!document.getElementById('cv-%[1]s'))return;T+=.016*S;cx.fillStyle='rgba(10,15,26,.26)';cx.fillRect(0,0,W,H);`+
		`for(var i=0;i<N;i++){var p=P[i],x=W/2+Math.cos(p.ph+T*p.w)*W*.3*A,y=H/2+Math.sin(p.ph*1.7+T*1.3*p.w)*H*.28*A;`+
		`cx.fillStyle=i%%2?C1:C2;cx.globalAlpha=.2+p.e*.65;cx.beginPath();cx.arc(x,y,1.5+4*p.e*A,0,7);cx.fill()}`+
		`cx.globalAlpha=1;requestAnimationFrame(loop)})()})()</script>`,
		d.name, d.density, d.speed, d.amp, d.c1, d.c2, d.caption())
}

// waves: stacked sine strands, phase-drifting at genome speed
func (d *dreamParams) famWaves() string {
	return fmt.Sprintf(`<style>#c-%[1]s canvas{display:block;margin:0 auto;border-radius:12px;background:#0a0f1a}</style>`+
		`<div id='c-%[1]s' style='text-align:center'><canvas id='cv-%[1]s' width='340' height='210'></canvas>`+
		`<div style="font-size:11px;color:#4d5570;margin-top:6px">%[7]s</div></div>`+
		`<script>(function(){var cv=document.getElementById('cv-%[1]s');if(!cv)return;var cx=cv.getContext('2d'),W=340,H=210,T=0,L=Math.max(8,Math.round(%[2]d/3)),S=%[3]f,A=%[4]f,C1='%[5]s',C2='%[6]s';`+
		`(function loop(){if(!document.getElementById('cv-%[1]s'))return;T+=.016*S;cx.fillStyle='rgba(10,15,26,.22)';cx.fillRect(0,0,W,H);`+
		`for(var i=0;i<L;i++){var y0=(i+.5)*H/L;cx.beginPath();cx.strokeStyle=i%%2?C1:C2;cx.globalAlpha=.12+.5*(1-Math.abs(i/L*2-1));`+
		`for(var x=0;x<=W;x+=6){var y=y0+Math.sin(x*.02+T*(1+(i%%5)*.25))*9*A*(.4+.6*Math.abs(Math.sin(i*.7)));x?cx.lineTo(x,y):cx.moveTo(x,y)}cx.stroke()}`+
		`cx.globalAlpha=1;requestAnimationFrame(loop)})()})()</script>`,
		d.name, d.density, d.speed, d.amp, d.c1, d.c2, d.caption())
}

// drift: particles riding a sin/cos flow field, wrapping the edges
func (d *dreamParams) famDrift() string {
	return fmt.Sprintf(`<style>#c-%[1]s canvas{display:block;margin:0 auto;border-radius:12px;background:#0a0f1a}</style>`+
		`<div id='c-%[1]s' style='text-align:center'><canvas id='cv-%[1]s' width='340' height='210'></canvas>`+
		`<div style="font-size:11px;color:#4d5570;margin-top:6px">%[7]s</div></div>`+
		`<script>(function(){var cv=document.getElementById('cv-%[1]s');if(!cv)return;var cx=cv.getContext('2d'),W=340,H=210,T=0,N=%[2]d,S=%[3]f,A=%[4]f,C1='%[5]s',C2='%[6]s',P=[];`+
		`for(var i=0;i<N;i++)P.push({x:Math.random()*W,y:Math.random()*H});`+
		`(function loop(){if(!document.getElementById('cv-%[1]s'))return;T+=.016;cx.fillStyle='rgba(10,15,26,.18)';cx.fillRect(0,0,W,H);`+
		`for(var i=0;i<N;i++){var p=P[i];p.x+=Math.sin(p.y*.015+T*S)*A*1.6;p.y+=Math.cos(p.x*.012+T*S*.8)*A*1.1;`+
		`if(p.x<0)p.x+=W;if(p.x>W)p.x-=W;if(p.y<0)p.y+=H;if(p.y>H)p.y-=H;`+
		`cx.fillStyle=i%%3?C1:C2;cx.globalAlpha=.5;cx.fillRect(p.x,p.y,2,2)}`+
		`cx.globalAlpha=1;requestAnimationFrame(loop)})()})()</script>`,
		d.name, d.density, d.speed, d.amp, d.c1, d.c2, d.caption())
}

// rings: concentric circles born at the center, breathing out
func (d *dreamParams) famRings() string {
	return fmt.Sprintf(`<style>#c-%[1]s canvas{display:block;margin:0 auto;border-radius:12px;background:#0a0f1a}</style>`+
		`<div id='c-%[1]s' style='text-align:center'><canvas id='cv-%[1]s' width='340' height='210'></canvas>`+
		`<div style="font-size:11px;color:#4d5570;margin-top:6px">%[7]s</div></div>`+
		`<script>(function(){var cv=document.getElementById('cv-%[1]s');if(!cv)return;var cx=cv.getContext('2d'),W=340,H=210,T=0,R=Math.max(6,Math.round(%[2]d/6)),S=%[3]f,A=%[4]f,C1='%[5]s',C2='%[6]s',MAX=Math.hypot(W,H)/2;`+
		`(function loop(){if(!document.getElementById('cv-%[1]s'))return;T+=.016*S;cx.fillStyle='rgba(10,15,26,.2)';cx.fillRect(0,0,W,H);`+
		`for(var i=0;i<R;i++){var r=((i*MAX/R)+T*26*A)%%MAX,a=1-r/MAX;cx.beginPath();cx.arc(W/2,H/2,4+r,0,7);`+
		`cx.strokeStyle=i%%2?C1:C2;cx.globalAlpha=a*.55+.05;cx.lineWidth=1+A;cx.stroke()}`+
		`cx.globalAlpha=1;requestAnimationFrame(loop)})()})()</script>`,
		d.name, d.density, d.speed, d.amp, d.c1, d.c2, d.caption())
}

// lines: vertical strands swaying on their own phase
func (d *dreamParams) famLines() string {
	return fmt.Sprintf(`<style>#c-%[1]s canvas{display:block;margin:0 auto;border-radius:12px;background:#0a0f1a}</style>`+
		`<div id='c-%[1]s' style='text-align:center'><canvas id='cv-%[1]s' width='340' height='210'></canvas>`+
		`<div style="font-size:11px;color:#4d5570;margin-top:6px">%[7]s</div></div>`+
		`<script>(function(){var cv=document.getElementById('cv-%[1]s');if(!cv)return;var cx=cv.getContext('2d'),W=340,H=210,T=0,L=Math.max(8,Math.round(%[2]d/4)),S=%[3]f,A=%[4]f,C1='%[5]s',C2='%[6]s';`+
		`(function loop(){if(!document.getElementById('cv-%[1]s'))return;T+=.016*S;cx.fillStyle='rgba(10,15,26,.2)';cx.fillRect(0,0,W,H);`+
		`for(var i=0;i<L;i++){var x0=(i+.5)*W/L;cx.beginPath();cx.strokeStyle=i%%2?C1:C2;cx.globalAlpha=.16+.45*(1-Math.abs(i/L*2-1));`+
		`for(var y=0;y<=H;y+=6){var x=x0+Math.sin(y*.03+T*(1+(i%%4)*.3))*10*A;y?cx.lineTo(x,y):cx.moveTo(x,y)}cx.stroke()}`+
		`cx.globalAlpha=1;requestAnimationFrame(loop)})()})()</script>`,
		d.name, d.density, d.speed, d.amp, d.c1, d.c2, d.caption())
}
