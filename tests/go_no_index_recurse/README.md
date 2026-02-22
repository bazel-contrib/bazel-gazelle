TestNoIndexRecurse checks that gazelle behaves correctly with the flags
-r=true -index=false. Gazelle should generate build files in directories
and subdirectories, but should not resolve dependencies to local libraries.
