import psutil
import sys
from subprocess import check_output
from time import sleep


def get_pid(name):
    return int(check_output(["pidof", "-s", name]))

def main():

    if len(sys.argv) < 2:
        print("Execute with the name of the choosen process")
        return

    try:
        out = open("io.out", "w", buffering=1)

        p = psutil.Process(get_pid(sys.argv[1]))
        print(p.name())

        while p.is_running():
            n = psutil.net_io_counters()
            #print(n.bytes_recv)
            #print(n.bytes_sent)
            out.write(str(n.bytes_recv) + "\n")
            out.write(str(n.bytes_sent) + "\n")

            i = p.io_counters()
            #print(i.read_bytes)
            #print(i.write_bytes)
            #print("")
            out.write(str(i.read_bytes) + "\n")
            out.write(str(i.write_bytes) + "\n\n")

            #print(p.cpu_percent())
            sleep(1)

    except Exception as e:
        print(e.__str__())
        return

if __name__ == "__main__":
    main()