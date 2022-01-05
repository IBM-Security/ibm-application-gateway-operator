#!/usr/bin/python

"""
Copyright contributors to the IBM Application Gateway Operator project

This script is designed to handle the management of the IAG operator
docker build image and containers.  The usage option can be used to provide 
details on the usage of this script.
"""

import sys
import subprocess
import argparse
import os

############################################################################
# Section: Global variables.

# Should we run in verbose mode or not?  Controlled by the '-v' command
# line argument.
verbose = False

# The name of our image.
image_name = "iag-operator-build"

############################################################################
# Section: utility functions

def execute(cmd, capture_out = False):
    """
    This function will execute the specified command.  If the command fails
    the program will exit.
    
    @param cmd         [in] : An array which represents the command to be 
                              executed
    @param capture_out [in] : Should we capture stdout?
    
    @retval stdout from the command
    """

    try:
        if verbose:
            print("  executing: " + " ".join(cmd))

        if capture_out:
            out = subprocess.check_output(cmd);
        else:
            out = subprocess.check_call(cmd)

        if verbose and capture_out:
            print("   output: \n" + out)

        return out

    except OSError:
        print("\nError> " + " ".join(cmd))
        sys.exit(1)
    except subprocess.CalledProcessError:
        print("\nError> " + " ".join(cmd))
        sys.exit(1)

def validate_source_path(source_path):
    """
    Validate that the provided source path exists and looks correct.
    """

    # It must be a valid directory.
    if not os.path.isdir(source_path):
        print("Error> the specified source path does not exist: {0}!".format(
                    source_path))

        sys.exit(1)

    go_module = os.path.join(source_path, "main.go")

    if not os.path.isfile(go_module):
        print("Error> the specified source path is invalid: {0}!".format(
                    source_path))

        sys.exit(1)

############################################################################
# Section: Commands for docker images

def build_command(args):
    """
    Handle the construction of the build image.  
    """

    if verbose:
        print("Command: build")

    # Now we can execute the build command.
    execute([
        "docker", "build", 
        "--force-rm", 
        "-t", "{0}:latest".format(image_name),
        os.path.dirname(os.path.realpath(__file__))
    ])

############################################################################
# Section: Commands on Docker containers

def create_command(args):
    """
    Handle the processing of the create command.  This will involve creating a
    new operator build container.
    """

    if verbose:
        print("Command: create")

    # Work out the source path, and then validate the source path.
    source = None

    if args.source:
        source = args.source
    else:
        source = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))

    validate_source_path(source)

    # Build up the docker command.  
    cmd  = [
            "docker", "run", "-t",  "--interactive",
            "--hostname", args.container, 
            "--name", args.container,
    ]

    # We need to add in our docker socket (if available).
    docker_sock = "/var/run/docker.sock"

    if os.path.exists(docker_sock):
        cmd.append("-v")
        cmd.append("{0}:{0}".format(docker_sock))

    # Add in our source path.
    cmd.append("--volume")
    cmd.append("{0}:/workspace".format(source))

    # Create a temporary file system to help speed up the build.
    cmd.append("--tmpfs")
    cmd.append("/tmp:rw,exec,dev,suid,relatime,mode=755")

    cmd.append(image_name + ":" + args.version)

    # Now we can create the docker container.
    execute(cmd)

def start_command(args):
    """
    Handle the processing of the start command.  
    """

    if verbose:
        print("Command: start")

    execute( ["docker", "start", "--attach",  "--interactive", args.container] )

############################################################################
# Section: Sundry Commands

def instructions_command(args):
    """
    Display the build environment set-up instructions.
    """

    print("""
Overview:
---------

This script can be used to manage a docker IAG operator build 
environment.  A single docker image can be created to encapsulate the
build environment.  A separate container can then be created from this image 
for each workspace which you wish to build.

When the container is first created you will be dropped into a shell from which
you can perform a build.  When you exit from the shell the container will be 
stopped.  When the container is next started you will simply be dropped
into a new shell, allowing you to build whatever you like.

Pre-Req:
--------

1. A recent version of docker must be installed and running.  The script has 
   been tested on a Macbook which is running docker v17.06.
2. The docker build image must be available in the environment.  The 'build' 
   command option for this script will automate the creation of the image.
3. You need a copy of the ibm-application-gateway-operator repository that you 
   are building.  

Options:
--------

The following options are available as a part of this script:

'build':  Used to construct the build image.  For example:
            'operator_build.py build'
'create': Used to create a new container for a specific workspace.  When the 
          container is first started you will be dropped into a shell.  The
          container will stop once you exit from the shell.  For example:
            'operator_build.py create --container build.operator \
                --source /Users/dev/github/ibm-application-gateway-operator'
'start':  Used to start a build container which is currently stopped.  You will
          be dropped into a shell in the build environment allowing you to 
          control which build commands are executed.  The container will stop
          once your exit from the shell.  For example:
            'operator_build.py start build.operator'
    """)

############################################################################
# Section: Argument Parser

def process_argv():
    """
    Create the command parser and then parse the arguments.
    """

    parser = argparse.ArgumentParser(description="This script is designed "
        "to help manage the IAG operator Docker build container.  At "
        "a high level you should construct the build image in your "
        "environment, and then you can start an individual container for each "
        "workspace that you want to build.")

    parser.add_argument("-v", "--verbose", help="Display the commands as " 
                "they are executed.", action="store_true")

    subparsers = parser.add_subparsers(help='sub-command help')

    # build command.....
    cmd = subparsers.add_parser("build", help="This command will " 
        "construct the IAG operator build image.")

    cmd.set_defaults(func=build_command)

    # create command....
    cmd = subparsers.add_parser("create", help="This command will create and "
        "start the docker container.  A shell will be established "
        "and the container will be stopped when the shell is exited.")

    cmd.add_argument("-v", "--version", help="The version of the build image "
        "(e.g 0.1). Default: latest", required=False, default="latest")

    cmd.add_argument("-s", "--source", help="The name of the directory "
        "which contains the ibm-application-gateway-operator source code.  "
        "Default: current path",
        required=False)

    cmd.add_argument("-c", "--container", help="The name of the container "
        "which will be created. Default: iag.operator.build",
        required=False, default="iag.operator.build")

    cmd.set_defaults(func=create_command)

    # start command...
    cmd = subparsers.add_parser("start", help="This command will start a "
        "pre-created docker container.  A shell will be established "
        "and the container will be stopped when the shell is exited.")

    cmd.add_argument("-c", "--container", help="The name of the container used "
        "for this build. Default: iag.operator.build",
        required=False, default="iag.operator.build")

    cmd.set_defaults(func=start_command)

    # instructions command...
    cmd = subparsers.add_parser("instructions", help="This command will "
        "display set-up instructions for the build environment.")

    cmd.set_defaults(func=instructions_command)

    return parser.parse_args()

############################################################################
# Section: Main Line

args    = process_argv()
verbose = args.verbose

args.func(args)

