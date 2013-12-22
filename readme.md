# Shipyard Agent
This is the Shipyard Agent.  It goes on your Docker hosts.

# Building
* `make`

# Usage
You first need to register with your Shipyard instance.  You can do this via:

`./shipyard-agent -url http://myshipyardhost -register`

It will output an "agent key".  You will then need to authorize the host in 
Shipyard.  Login to your Shipyard instance and select "Hosts".  Click on the 
action menu for the host and select "Authorize Host".

Once authorized, you can start the agent:

`./shipyard-agent -url http://myshipyardhost -key 1234567890qwertyuiop`

