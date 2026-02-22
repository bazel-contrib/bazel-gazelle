go_no_index_no_recurse checks that gazelle behaves correctly with the flags
-r=false -index=false. Gazelle should not generate build files in
subdirectories and should not resolve dependencies to local libraries.
