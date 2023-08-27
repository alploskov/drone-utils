from pathlib import Path
import typer
from typer import Option
from pymavlink import mavutil
from .mavftp import FTP

app = typer.Typer()

@app.command()
def main(addr: str = Option(default='/dev/ttyACM0'),
         script: str = Option(default=''),
         is_sitl: bool = Option(False, '--sitl/--no-sitl')):
    conn = mavutil.mavlink_connection(addr)
    conn.wait_heartbeat()
    print("Heartbeat from APM (system %u component %u)" % (conn.target_system, conn.target_system))
    ftp_wrapper = FTP(conn)
    if script != '':
         script_name = Path(script).name
         target_dir = Path('/scripts' if is_sitl else '/APM/scripts')
         print(f'Upload {script_name} to {addr} {target_dir/script_name}')
         ftp_wrapper.write_file(script, target_dir/script_name)
    cmd = conn.mav.command_int_encode(
        conn.target_system,      # Target system ID
        conn.target_component,   # Target component ID
        0,
        mavutil.mavlink.MAV_CMD_SCRIPTING,
        0, 0,                    # current, autocontinue
        mavutil.mavlink.SCRIPTING_CMD_STOP_AND_RESTART,
        0, 0, 0,                 # Params 2, 3, 4 (unused)
        0, 0, 0                  # Params 5, 6, 7 (unused) 
    )
    conn.mav.send(cmd)
    response = conn.recv_match(type='COMMAND_ACK', blocking=True)
    if response and response.result == mavutil.mavlink.MAV_RESULT_ACCEPTED:
        print(f'Restart lua scripts in {addr}')
    else:
        print('Restart lua scripts failed')
