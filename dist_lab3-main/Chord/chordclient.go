package Chord

import (
	"flag"
	"fmt"
	"regexp"
)

type Flags struct {
	IP              string //ValidInputNew[0]
	Port            int    //ValidInputNew[1]
	JA              string //ValidInputJoin[0]
	JP              int    //ValidInputJoin[1]
	Ts              int    //ValidInputOther[0]
	Tff             int    //ValidInputOther[1]
	Tcp             int    //ValidInputOther[2]
	R               int    //ValidInputOther[3]
	UserID          string //ValidInputOther[4]
	M               int    //ValidInputOther[5]
	ValidInputNew   [2]bool
	ValidInputJoin  [2]bool
	ValidInputOther [6]bool
}

var flags Flags

/*
Start checks the argument given by the user via handelFlags(). If any given flag is invalid the function break.
Depending on the arguments given a new node is created and a new ring is started or joined.
*/

func Start(args []string) {

	handelFlags(args)

	if checkValidInputNew() && checkValidInputJOther() {
		if checkValidInputJoin() {
			fmt.Printf("JOIN a RING\n")

			createNode(flags, false)

		} else {
			fmt.Printf("CREATE a RING\n")

			createNode(flags, true)

		}
	} else {

		fmt.Printf("Error. All flags where not given in a correct way\n")
		return
	}
}

func handelFlags(args []string) {
	//			:place to save    :flag-name  :default-value    :info text if -h is given
	flag.StringVar(&flags.IP, "a", "", "Specify IP-adress")
	flag.IntVar(&flags.Port, "p", 0, "Specify Portnumber")
	flag.StringVar(&flags.JA, "ja", "", "JOIN given ip address. Must be given if --ja is used")
	flag.IntVar(&flags.JP, "jp", 0, "JOIN given port. Must be given if --jp is used")
	flag.IntVar(&flags.Ts, "ts", 0, "The time in milliseconds between invocations of 'stabilize'. Range [1,60000]")
	flag.IntVar(&flags.Tff, "tff", 0, "The time in milliseconds between invocations of 'fix fingers'. Range [1,60000]")
	flag.IntVar(&flags.Tcp, "tcp", 0, "The time in milliseconds between invocations of 'check predecessor'. Range [1,60000]")
	flag.IntVar(&flags.R, "r", 0, "Number of successors maintained by the Chord client. Range [1,32]")
	flag.StringVar(&flags.UserID, "i", "", "The identifier (ID) assigned to the Chord client: string of 40 characters matching [0-9a-fA-F]")
	flag.IntVar(&flags.M, "m", 0, "The size of the ring, must be give [1 - 20]")

	// Parse flag from commandLine
	flag.CommandLine.Parse(args)

	//A-flag
	if flags.IP != "" {
		fmt.Printf("Given IP-adress: %s\n", flags.IP)
		flags.ValidInputNew[0] = true
	} else {
		fmt.Printf("Specify IP-adress!!\n")
		flags.ValidInputNew[0] = false
		return
	}

	//P-flag
	if flags.Port != 0 {
		fmt.Printf("Given Portnumber: %d\n", flags.Port)
		flags.ValidInputNew[1] = true
	} else {
		fmt.Printf("Specify Portnr!!\n")
		flags.ValidInputNew[1] = false
		return
	}

	//JA-flag & JP-flag

	if flags.JA != "" {
		fmt.Printf("JOIN given ip address: %s\n", flags.JA)
		flags.ValidInputOther[5] = true //If ARGUMENTfor join is given, we set the M flag to true.
		flags.ValidInputJoin[0] = true
	} else {
		flags.ValidInputJoin[0] = false

	}

	if flags.JP != 0 {
		fmt.Printf("JOIN given port: %d\n", flags.JP)
		flags.ValidInputOther[5] = true //If ARGUMENT for join is given, we set the M flag to true.
		flags.ValidInputJoin[1] = true
	} else {
		flags.ValidInputJoin[1] = false
	}

	if (flags.ValidInputJoin[1] && !flags.ValidInputJoin[0]) || (flags.ValidInputJoin[0] && !flags.ValidInputJoin[1]) {
		fmt.Printf("JA and JP flag must be given together\n")
		return
	}

	//TS-flag

	if flags.Ts >= 1 && flags.Ts <= 60000 {
		fmt.Printf("Time between 'stabilize' invocations: %d\n", flags.Ts)
		flags.ValidInputOther[0] = true
	} else {
		fmt.Println("Error: 'ts' value out of range. Range [1,60000]")
		flags.ValidInputOther[0] = false
		return
	}

	//TFF-flag

	if flags.Tff >= 1 && flags.Tff <= 60000 {
		fmt.Printf("Time between 'fix fingers' invocations: %d\n", flags.Tff)
		flags.ValidInputOther[1] = true
	} else {
		fmt.Println("Error: 'tff' value out of range. Range [1,60000]")
		flags.ValidInputOther[1] = false
		return
	}

	//TCP-flag

	if flags.Tcp >= 1 && flags.Tcp <= 60000 {
		fmt.Printf("Time between 'check predecessor' invocations: %d\n", flags.Tcp)
		flags.ValidInputOther[2] = true
	} else {
		fmt.Println("Error: 'tcp' value out of range. Range [1,60000]")
		flags.ValidInputOther[2] = false
		return
	}

	// R-flag

	if flags.R >= 1 && flags.R <= 32 {
		fmt.Printf("Number of successors: %d\n", flags.R)
		flags.ValidInputOther[3] = true
	} else {
		fmt.Println("Error: 'r' value out of range. Range [1,32]")
		flags.ValidInputOther[3] = false
		return
	}

	//I-flag OPTIONAL

	flags.ValidInputOther[4] = true //Since optional
	if flags.UserID != "" {
		match, err := regexp.MatchString("^[0-9a-zA-Z]{0,40}$", flags.UserID)
		if CheckError(err, "Error during regexp check on UserID") {
			return
		}
		if match {
			// Om det är en match, hantera strängen
			fmt.Printf("Match! Value: %s\n", flags.UserID)
		} else {
			// Om det inte är en match, generera ett felmeddelande
			fmt.Printf("Error: 'UserID' value must be a string of up to 40 characters matching [0-9a-zA-Z]")
			flags.ValidInputOther[4] = false
			return
		}
	}

	//M flag  (M is for ringsize)

	if flags.M != 0 {
		if flags.M >= 1 && flags.M <= 160 {
			if flags.JP == 0 && flags.JA == "" {
				fmt.Printf("M: %d\n", flags.M)
				flags.ValidInputOther[5] = true
			} else {
				fmt.Println("Do not specify the 'm' flag if the flags for join ('jp' & 'ja') are provided.")
				flags.ValidInputOther[5] = false
				return
			}
		} else {
			fmt.Println("Error: 'm' ring size value out of range. Range [1,160]")
			flags.ValidInputOther[5] = false
		}
	} else if flags.M == 0 && flags.JP == 0 && flags.JA == "" {
		flags.ValidInputOther[5] = false
		fmt.Println("The M flag needs to be specified when creating a new ring")
	}
}

/*
checkValidInputNew checks if the argument (-a) IP and (-p) Port is valid.
If they are, a new ring could be either created and joined.
*/
func checkValidInputNew() bool {

	for _, value := range flags.ValidInputNew {

		if value != true {

			return false
		}
	}
	return true
}

/*
checkValidInputJoin checks if the argument (-ja) JoinIP and (-jp) JoinPort is valid.
If they are, a ring could be joined.
*/
func checkValidInputJoin() bool {

	for _, value := range flags.ValidInputJoin {

		if value != true {

			return false
		}
	}
	return true
}

/*
checkValidInputJOther checks if the argument Ts, tff, tcp, r , i (userId) & m is valid.
Since -i is optional it's always valid if it's not given. M flag can only be valid if
-ja and -jp is not given. A user cannot join a ring and specify a different ringsize.
*/
func checkValidInputJOther() bool {

	for _, value := range flags.ValidInputOther {

		if value != true {
			return false
		}
	}
	return true
}

/*
CheckError prints error messages and the specefied place where it occured
*/
func CheckError(error error, place string) bool {
	if error != nil {
		if CheckErrorprint {
			fmt.Printf("\x1b[31mError during:%s.\x1b[0m \n", place)
			fmt.Printf("\x1b[31mexplanation Err =:%s.\x1b[0m \n", error)
		}
		return true
	} else {
		return false
	}
}
