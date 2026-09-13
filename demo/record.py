"""Record demo/hlw.gif: drive hlw in a real PTY, snapshot the screen with
pyte, render each frame with PIL, then encode with ffmpeg.

Run it through demo/record.sh rather than directly.

This exists instead of a vhs tape because vhs 0.12 produced no output on the
machine this was recorded on. Driving the PTY directly also makes the cut
point exact: recording stops at hlw's hand-off, which is where its job ends.
"""
import os, pty, sys, time, select, fcntl, termios, struct, json
import pyte
from PIL import Image, ImageDraw, ImageFont

ROWS, COLS = int(os.environ.get("ROWS", 22)), int(os.environ.get("COLS", 96))
FPS = 10
FONT = "/System/Library/Fonts/Menlo.ttc"
FS = 22
OUT = os.environ["FRAMEDIR"]

# Catppuccin-ish dark palette
BG, FG = (30, 30, 46), (205, 214, 244)
ANSI = {
    "black": (69, 71, 90), "red": (243, 139, 168), "green": (166, 227, 161),
    "brown": (249, 226, 175), "yellow": (249, 226, 175), "blue": (137, 180, 250),
    "magenta": (245, 194, 231), "cyan": (148, 226, 213), "white": (205, 214, 244),
    "default": FG,
}
def color(name, default):
    if name in ANSI: return ANSI[name]
    if isinstance(name, str) and len(name) == 6:
        try: return tuple(int(name[i:i+2], 16) for i in (0, 2, 4))
        except ValueError: return default
    return default

font = ImageFont.truetype(FONT, FS)
bold = ImageFont.truetype(FONT, FS, index=1)
bbox = font.getbbox("M")
CW = int(font.getlength("M"))
CH = int(FS * 1.32)
PAD = 18
W, H = COLS * CW + PAD * 2, ROWS * CH + PAD * 2

def render(screen, path):
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    for y in range(ROWS):
        line = screen.buffer[y]
        for x in range(COLS):
            ch = line[x]
            if ch.bg != "default":
                d.rectangle([PAD + x*CW, PAD + y*CH, PAD + (x+1)*CW, PAD + (y+1)*CH],
                            fill=color(ch.bg, BG))
            if ch.data and ch.data != " ":
                d.text((PAD + x*CW, PAD + y*CH), ch.data,
                       font=bold if ch.bold else font, fill=color(ch.fg, FG))
    img.save(path)

def main():
    steps = json.load(open(os.environ["SCRIPT"]))
    env = dict(os.environ, TERM="xterm-256color", LINES=str(ROWS), COLUMNS=str(COLS),
               HOME=os.environ["DEMOHOME"], PATH=os.environ["DEMOPATH"])
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(os.environ["DEMOHOME"])
        env["PS1"] = "$ "
        os.execvpe("/bin/sh", ["/bin/sh"], env)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))

    screen = pyte.Screen(COLS, ROWS); stream = pyte.ByteStream(screen)
    frames = [0]
    def snap():
        i = frames[0]
        render(screen, os.path.join(OUT, "f%05d.png" % i))
        with open(os.path.join(OUT, "f%05d.txt" % i), "w") as fh:
            fh.write("\n".join(screen.display))
        frames[0] += 1
    def pump(seconds):
        end = time.time() + seconds
        nxt = time.time()
        while time.time() < end:
            r, _, _ = select.select([fd], [], [], 0.02)
            if r:
                try: data = os.read(fd, 65536)
                except OSError: return
                if not data: return
                stream.feed(data)
                if b"\x1b]11;?" in data: os.write(fd, b"\x1b]11;rgb:1e1e/1e1e/2e2e\x1b\\")
                if b"\x1b[6n" in data:   os.write(fd, b"\x1b[1;1R")
            if time.time() >= nxt:
                snap(); nxt += 1.0 / FPS
    KEYS = {"enter": b"\r", "down": b"\x1b[B", "up": b"\x1b[A", "ctrlc": b"\x03"}
    for step in steps:
        if "type" in step:
            for c in step["type"]:
                os.write(fd, c.encode()); pump(step.get("speed", 0.045))
        if "key" in step:
            os.write(fd, KEYS[step["key"]])
        if "wait" in step:
            pump(step["wait"])
        if step.get("until"):
            deadline = time.time() + 6
            while time.time() < deadline:
                pump(0.1)
                text = "\n".join(screen.display)
                if step["until"] in text:
                    pump(0.25)
                    break
    import signal
    try: os.killpg(os.getpgid(pid), signal.SIGKILL)
    except Exception:
        try: os.kill(pid, signal.SIGKILL)
        except Exception: pass
    try: os.waitpid(pid, 0)
    except Exception: pass
    try: os.close(fd)
    except OSError: pass
    print("frames:", frames[0], "size: %dx%d" % (W, H))

main()
