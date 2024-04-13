package Chord

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"path"
	"strconv"
	"time"
)

const MaxSteps = 32

const Debugging = false
const CheckErrorprint = false

type Node struct {
	Id          big.Int  //
	Address     string   //ipadress:port
	FingerTable []string //
	Predecessor string   //The previous node on the identifier circle
	Successors  []string //-r [1,32]
	Bucket      map[string][]string
	Flags       Flags
	M2          big.Int
	M           int
	stopChan    chan struct{}
}

/*
Creates a new node and joins an if the ring exists it will join the ring, or if not
It will create a new ring
*/
func createNode(flags Flags, createNewRing bool) {
	fmt.Printf("Node Started with ip %s, port %d\n", flags.IP, flags.Port)

	n := Node{}
	n.Flags = flags //Set node flags
	go n.server()   //start server
	time.Sleep(200 * time.Millisecond)

	//How to set M on whether it's a new ring or a join
	if !createNewRing { //If JOIN
		n.GetM(n.Flags.JA, n.Flags.JP) //Get M from the Node we are joining.
	} else { //IF Create
		n.M = flags.M
	}
	//set node adress
	n.Address = flags.IP + ":" + strconv.Itoa(flags.Port)
	n.M2 = *n.calculateM2()
	//Calculate and set node ID based on adress
	n.Id = *hashModulo(Hash(n.Address), n.M2)
	n.Successors = make([]string, n.Flags.R) //The size of Successors is n.Flags.R
	n.FingerTable = make([]string, n.M)
	n.Bucket = make(map[string][]string)
	n.stopChan = make(chan struct{})

	if createNewRing { //Create new Ring
		n.create()
	} else { //Join Ring
		n.join(n.Flags.JA, n.Flags.JP) //ja = ip to join, jp = port to join.
	}

	n.PrintDetails()

	//Start the periodical functions in separate go routines
	go n.fix_fingers(n.Flags.Tff)
	go n.check_predecessor(n.Flags.Tcp) // Check predecessor with interval Tcp
	go n.stabilize(n.Flags.Ts)          // Stabilize the ring with interval Ts

	n.InputLoop()
}

/*
Calculates 2^n.M. returns is as a big.int
*/
func (n *Node) calculateM2() *big.Int {
	return new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(n.M)), nil)
}

/*
InputLoop catches user input. Loops until given command Exit.
*/
func (n *Node) InputLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Give a command: \n")
		scanner.Scan()
		command := scanner.Text()

		switch command {

		case "Closepred":
			scanner.Scan()
			ID := scanner.Text()
			IDBigInt := new(big.Int)
			IDBigInt, _ = IDBigInt.SetString(ID, 10)

			addressGiven := n.closestPrecedingNode(*IDBigInt)

			fmt.Printf("The address returned is: %s\n", addressGiven)

		case "Lookup":
			fmt.Print("Lookup: Give a filename: ")
			scanner.Scan()
			fileId, fileHost := n.Lookup(scanner.Text())
			fmt.Printf("FIleID %s, stored at FileHost: %s\n", fileId.String(), fileHost)

		case "StoreFile":
			fmt.Println("StoreFile: Give file path:")
			scanner.Scan()
			n.StoreFile(scanner.Text())

		case "PrintState":
			n.PrintDetails()
		case "Exit":

			fmt.Println("Program is exiting.")
			n.Exit()
		default:
			fmt.Println("Invalid command. Use Lookup, StoreFile, or PrintState. Type 'Exit' to exit.")
		}
	}
}

/*
Lookup takes a filename as input. Hashes it with SHA-1 and runs modulus with Ringsize to calculate an ID.
The runs find on the chord ring to find the successor of that ID. That ID is responsible for storing the file.
*/
func (n *Node) Lookup(fileName string) (big.Int, string) {
	FileID := hashModulo(Hash(fileName), n.M2)

	found, suc := n.find(*FileID, n.Address, MaxSteps)
	if found {

		fmt.Printf("FileId %s, (Should be) stored at node: %s,\n ", FileID.String(), suc)
		//}
		return *FileID, suc //Suc = address
		//"The Chord client then outputs that node’s identifier, IP address, and port."
	} else {
		fmt.Printf("Max steps reached during Lookup\n")
	}

	return *FileID, "No Suc Found During Lookup"
}

/*
StoreFile, Takes a filepath. Reads the file from disk. Runs func Lookup on the filename. Sends the file
to the resonsible node found by Lookup.
*/
func (n *Node) StoreFile(filePath string) {

	fileName := path.Base(filePath) //Using path.Base to get the filename separated from the path.
	FileID, fileOwner := n.Lookup(fileName)

	content, readok := readFile(filePath)

	if readok {
		// read the content from the file to a []byte
		fileStruct := File{ID: FileID, FileName: fileName, Content: content}
		//Send message
		SenderArgsNotify := SendArgs{StoreFileRequest: true, File: fileStruct}
		ReceiveArgs := ReceiveArgs{}
		ok := n.call("Node.CallHandler", &SenderArgsNotify, &ReceiveArgs, fileOwner)

		if ok {
			//fmt.Printf("Reply from %s : %s\n", fileOwner, ReceiveArgs.ReplyArgs)
		} else {
			fmt.Printf("Error during call in StoreFile\n")
		}
	} else {
		fmt.Printf("No such file on disk in store file\n")
	}
}

// Read the content of a file and return it with a success status
func readFile(filePath string) ([]byte, bool) {

	file, err := os.Open(filePath)
	defer file.Close()
	if CheckError(err, "Readfile in StoreFile") {
		return nil, false
	}

	content, err := ioutil.ReadAll(file)
	if CheckError(err, "ReadAll in StoreFile") {
		return nil, false
	}

	return content, true
}

/*
Exit sends all the files in the current nodes bucket to its successor.
Then Closes down all the processes on the current node. And deletes the files from the disk.
*/

func (n *Node) Exit() {

	if hashModulo(Hash(n.Successors[0]), n.M2).Cmp(&n.Id) == 0 { //I´m the only one

		n.deleteDirectory("bucket" + n.Id.String())
		println("No need to send the files, no other Node in ring: EXIT")
		close(n.stopChan) //Closing down all threads.
		time.Sleep(1 * time.Second)
		os.Exit(1)
	} else {

		Filebucket := make(map[string][]File)

		for key, fileNames := range n.Bucket {
			//Loop for all the files in every katalog
			for _, fileName := range fileNames {
				filepath := fmt.Sprintf("%s/%s/%s", "bucket"+n.Id.String(), key, fileName)
				content, readok := readFile(filepath)

				if readok {
					Key := new(big.Int)
					Key.SetString(key, 10)

					file := File{ID: *Key, FileName: fileName, Content: content}
					Filebucket[key] = append(Filebucket[key], file)
				} else {
					fmt.Printf("No such file on disk in exit\n")
				}
			}
			delete(n.Bucket, key)
		}

		SenderArgsNotify := SendArgs{PutAllRequest: true, SendBucket: Filebucket}
		ReceiveArgs := ReceiveArgs{}
		ok := n.call("Node.CallHandler", &SenderArgsNotify, &ReceiveArgs, n.Successors[0])
		if ok {
			if ReceiveArgs.Answer {
				println("OK with Exit")
				close(n.stopChan) //Closing down all threads.
				n.deleteDirectory("bucket" + n.Id.String())
				time.Sleep(1 * time.Second)
				os.Exit(1)
			}
		} else {
			println("Error during call in PutAll")
		}
	}
}

/*
GetM makes a call to an Node address and retreives the value M (2^M = ringsize)
Adds the fetched M to the current nodes struct.
*/
func (n *Node) GetM(ja_ip string, jp_port int) {
	calladdress := ja_ip + ":" + strconv.Itoa(jp_port)

	SenderArgs := SendArgs{Mrequest: true}
	ReceiveArgs := ReceiveArgs{}
	ok := n.call("Node.CallHandler", &SenderArgs, &ReceiveArgs, calladdress)

	if ok { //The call failed, Meaning the n.predecessor has Failed/Crashed
		n.M = ReceiveArgs.ReplyInt
	} else {
		println("Error during call in GetM")
		os.Exit(1) //If we cannot get the M value we should abourt.
	}

}

/*
Creates an HTTP server listening on tcp on the port given by flag -p
*/
func (n *Node) server() {
	rpc.Register(n)
	rpc.HandleHTTP()
	addr := fmt.Sprintf(":%d", n.Flags.Port)

	l, e := net.Listen("tcp", addr)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	fmt.Printf("Listening on port %d...\n", n.Flags.Port)

	select {
	case <-n.stopChan:
		fmt.Println("Stoping server")
		l.Close()
	default:

		http.Serve(l, nil)
	}
}

// Create a new Chord ring with the currnet node as the only one in the ring
func (n *Node) create() {
	//Set predecessor of the current node to its adress
	n.Predecessor = n.Address
	//Set first successor to its adress
	n.Successors[0] = n.Address
	//The same for the first entry in FingerTable
	n.FingerTable[0] = n.Address
}

// Join a chord ring containing node with address given by -ja & -jp
// Runs find until it finds its successor on the chord ring.
// Asks its successor if there is any files it should  be responsible for,
// in that is the case, it retrives the files and stores them on disk
func (n *Node) join(ja_ip string, jp_port int) {

	n.Predecessor = ""

	calladdress := ja_ip + ":" + strconv.Itoa(jp_port)

	fmt.Printf("Joining Node with adress %s\n", calladdress)

	found, successor := n.find(n.Id, calladdress, MaxSteps)
	if found {
		n.Successors[0] = successor
		n.FingerTable[0] = successor

		SenderArgsNotify := SendArgs{GetAllRequest: true, SendArg: n.Id}
		ReceiveArgs := ReceiveArgs{}
		ok := n.call("Node.CallHandler", &SenderArgsNotify, &ReceiveArgs, n.Successors[0]) //CALL OUR SUCCESSOR AND ASK FOR THE FILES WE SHOULD BE RESPONSIBLE FOR
		if ok {
			//fmt.Printf("%s sent a GetAllRequest and the call was ok \n", n.Id.String())

			n.putAll(ReceiveArgs.SendBucket)
		} else {
			println("Error during call FOR FILES in join")
		}
	} else {
		fmt.Println("Join failed, MaxStep override")
	}
}

// stabilize is called periodically. Verifies the nodes immediate successor and tells the successor about n.
// First checks if the successors predecessor is between the current node and the successor, if so, the current node have
// a new successor and updates its successorList. Also notifies its new successors about the possibility of beeing its predecessor.
func (n *Node) stabilize(ts int) {
	duration := time.Duration(ts) * time.Millisecond

	/*A modified version of the stabilize procedure in Figure 6 maintains
	the successor list. Successor lists are stabilized as follows: node n reconciles its list with
	its successor s by copying s’s successor list, removing its last
	entry, and prepending s to it. If node n notices that its successor
	has failed, it replaces it with the first live entry in its successor
	list and reconciles its successor list with its new successor. At
	that point, n can direct ordinary lookups for keys for which the
	failed node was the successor to the new successor.
	*/
	for {
		select {
		case <-n.stopChan:
			fmt.Println("Stopping stabilize thread")
			return
		default:
			time.Sleep(duration)

			if Debugging {
				fmt.Printf("\nstabilize\n")
			}

			SenderArgsPred := SendArgs{GetPredecessorRequest: true} //The argument to send to the node we are joining is the current nodes address.
			ReceiveArgsPred := ReceiveArgs{}

			ok := n.call("Node.CallHandler", &SenderArgsPred, &ReceiveArgsPred, n.Successors[0])

			if ok {

				SuccID := hashModulo(Hash(n.Successors[0]), n.M2)
				x := hashModulo(Hash(ReceiveArgsPred.ReplyArgs), n.M2)

				if ReceiveArgsPred.ReplyArgs != "" && between(&n.Id, x, SuccID, false) { //If my successor's predecessor is located between me and my successor, it becomes my new successor.
					n.Successors[0] = ReceiveArgsPred.ReplyArgs
				}

				//Getting the successor list from our (could be new) successor.

				SenderArgsPred := SendArgs{GetSuccessorListRequest: true} //The argument to send to the node we are joining is the current nodes address.
				ReceiveArgsPred := ReceiveArgs{}

				ok = n.call("Node.CallHandler", &SenderArgsPred, &ReceiveArgsPred, n.Successors[0])

				if ok {
					newSuccessors := make([]string, len(n.Successors))
					//fmt.Println("We are updating our Successors list: the length of the successor list: ", len(newSuccessors))
					copy(newSuccessors[1:], ReceiveArgsPred.SuccessorList[:len(n.Successors)-1]) //Copy with a shift of one position.

					newSuccessors[0] = n.Successors[0] //The first position should be replaced by our successo

					n.Successors = newSuccessors //Updte the list
				} else {
					fmt.Printf("Error during call in stabilize\n")
				}

			} else {
				//Our suc is dead: Replace it with the successor[1] if that is alive, and update the table.
				if Debugging {
					fmt.Printf("Our successor is dead, the first call to check Pred failed, now we check if others in our Succlist is alive\n")
				}

				for i := 1; i < len(n.Successors); i++ {
					if n.isNodeAlive(n.Successors[i]) {
						n.Successors[0] = n.Successors[i]

						var x = 1
						for y := i + 1; y < len(n.Successors); y++ {
							n.Successors[x] = n.Successors[y]
							n.Successors[y] = "" //Clear the position we have moved.
							x++
						}
						break
					}
				}
			}
			//Process to notify
			SenderArgsNotify := SendArgs{Notify: true, SendArgString: n.Address} //The argument to send to the node we are joining is the current nodes address.
			ok = n.call("Node.CallHandler", &SenderArgsNotify, nil, n.Successors[0])
			if !ok {
				fmt.Printf("Inside Stabilize: Error during Nofity call\n")
			}
		}
	}
}

/*
Check if the node at the provided adress is alive
If the call of CheckSucORPredFail fails, it indicates the predecessor chas failed/crashed
If the call succeeds with "all_good" reply, the node is alive
*/
func (n *Node) isNodeAlive(address string) bool {

	SenderArgs := SendArgs{CheckSucORPredFail: true}
	ReceiveArgs := ReceiveArgs{}
	// Call the RPC and check for the result of calling CheckSucORPredFail
	ok := n.call("Node.CallHandler", &SenderArgs, &ReceiveArgs, address)

	if !ok { //The call failed, Meaning the n.predecessor has Failed/Crashed
		if Debugging {
			fmt.Printf("The succsessor seems to have Failed\n")
		}
		return false

	} else if ReceiveArgs.ReplyArgs == "all_good" { // If the call succeeded with "all_good" reply
		return true
	}
	return true
}

// The Node with address Var. address thinks it might be our predecessor. If the incoming address is between us and our old predecessor,
// or the current node doesn't have any precedecessor, we update our predecessor to the new address.
func (n *Node) notify(address string) {

	if n.Predecessor == address { //If we already know the pred we don´t have to do anything.
		return
	}

	addressID := hashModulo(Hash(address), n.M2)

	var PredecessorAsBigInt big.Int

	PredecessorAsBigInt = *hashModulo(Hash(n.Predecessor), n.M2)

	// If Predecessor is not specified OR if both the address we receive is not equal to our current Predecessor AND if
	//the address is between our previous predecessor and us, then the address becomes our new predecessor.
	if n.Predecessor == "" || (address != n.Predecessor && between(&PredecessorAsBigInt, addressID, &n.Id, false)) {

		n.Predecessor = address //Uppdate the predecessor with new address
		//fmt.Printf("Updating my pred\n")
	}
}

/*
Fix_fingers is called periodically. Refreshes finger tables entries. Next stores the index of the next fingers to fix.
Calls the jump function which calculates the "jump" length which is 2^(next -1) and runs find to retrieve the closest successor to that ID.
*/
func (n *Node) fix_fingers(tff int) {

	duration := time.Duration(tff) * time.Millisecond
	next := 0

	for {
		select {
		case <-n.stopChan:
			fmt.Println("Stopping fix fingers")
			return
		default:
			time.Sleep(duration)
			if Debugging {
				fmt.Printf("\nFix_fingers\n")
			}
			next = next % n.M

			next++

			fingerStart := n.jump(n.Id, next)
			//find the successor for the current
			found, suc := n.find(*fingerStart, n.Address, MaxSteps)
			if found {
				n.FingerTable[next-1] = suc
			} else {
				fmt.Printf("Max steps reached\n")
			}
		}
	}
}

// Called periodically. Checks whether predecessor has failed.
// Does nothing if the predecessor is empty. Otherwise it calls to its predecessor and
// awaits an "all_good" back. If no responce, we asume the predeccessor is dead and we set it to empty.
func (n *Node) check_predecessor(tcp int) {
	duration := time.Duration(tcp) * time.Millisecond

	for {
		select {
		case <-n.stopChan:
			fmt.Println("Stopping check predecessor")
			return
		default:
			time.Sleep(duration)
			if Debugging {
				fmt.Printf("\nCheck_predecessor\n")
			}

			if n.Predecessor == "" {
				continue //Loop again and sleep
			}

			SenderArgs := SendArgs{CheckSucORPredFail: true}
			ReceiveArgs := ReceiveArgs{}
			ok := n.call("Node.CallHandler", &SenderArgs, &ReceiveArgs, n.Predecessor)

			if !ok { //The call failed, Meaning the n.predecessor has Failed/Crashed

				fmt.Printf("The Predecessor seems to have Failed\n")
				n.Predecessor = ""
			}
			if Debugging {
				fmt.Printf("Predecessor is ok\n")
			}
		}
	}
}

/*
Check if a given value "elt" is between two values "start" and "end"
considering if the range is,
inclusive by indicating whether the range is inclusive (true) or exclusive (false)
returns true if "elt" is between "start" and "end", otherwise false
*/

func between(start, elt, end *big.Int, inclusive bool) bool {
	if end.Cmp(start) > 0 {
		return (start.Cmp(elt) < 0 && elt.Cmp(end) < 0) || (inclusive && elt.Cmp(end) == 0)
	} else {
		return start.Cmp(elt) < 0 || elt.Cmp(end) < 0 || (inclusive && elt.Cmp(end) == 0)
	}
}

/*
Creates a big.ing hashvalue of a given string with sha1 function. Returns the value as a big.int
*/
func Hash(address string) big.Int {
	hasher := sha1.New()
	hasher.Write([]byte(address))
	hashInt := new(big.Int).SetBytes(hasher.Sum(nil))
	return *hashInt
}

/*
Runs modulo, hash % ringsize. Where both input are big.int. Returns a big.int pointer.
*/
func hashModulo(hash big.Int, ringSize big.Int) *big.Int {
	result := new(big.Int).Mod(&hash, &ringSize)
	return result

}

/*
Makes a call (DialHTTP) to given Node on address. Returns true if call was successfull, otherwise returns false.
*/
func (n *Node) call(rpcname string, args interface{}, reply interface{}, adress string) bool {
	if Debugging {
		fmt.Printf("In call function: Calling from %s to %s\n", n.Address, adress)
	}
	c, err := rpc.DialHTTP("tcp", adress) //adress = "ip:port" (correct format)

	if CheckError(err, "During rpc.DialHTTP, in call-function") {
		return false
	}

	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}
	fmt.Println(err)
	return false
}

/*
CallHandler is the target function for all calls made by Nodes. It checks what kind of request that is made
and starts appropriate appropriate function. Returns arguments to the caller.
*/

func (n *Node) CallHandler(sendArgs *SendArgs, receiveArgs *ReceiveArgs) error {

	if sendArgs.GetSuccessorRequest { //When find() calls to find a succ
		bool, address := n.findSuccessor(sendArgs.SendArg)
		receiveArgs.FindSuccessorAnswer.IsSuccessor = bool
		receiveArgs.FindSuccessorAnswer.Address = address
		receiveArgs.Answer = true

	} else if sendArgs.GetPredecessorRequest { //When stabilize() calls to find pred
		receiveArgs.ReplyArgs = n.Predecessor
		receiveArgs.Answer = true

	} else if sendArgs.Notify { //When notify() calls
		n.notify(sendArgs.SendArgString)

	} else if sendArgs.CheckSucORPredFail { //When check_predecessor() calls and alive in stabilize
		receiveArgs.ReplyArgs = "all_good"
		receiveArgs.Answer = true

	} else if sendArgs.Mrequest {
		receiveArgs.ReplyInt = n.M
		receiveArgs.Answer = true
	} else if sendArgs.StoreFileRequest {

		n.putFile(sendArgs.File)
		receiveArgs.ReplyArgs = "File Stored"
		receiveArgs.Answer = true
	} else if sendArgs.GetSuccessorListRequest {

		receiveArgs.Answer = true
		receiveArgs.SuccessorList = n.Successors
	} else if sendArgs.PutAllRequest {

		n.putAll(sendArgs.SendBucket)
		receiveArgs.Answer = true
	} else if sendArgs.GetAllRequest {
		if len(n.Bucket) != 0 {
			receiveArgs.SendBucket = n.getAll(&sendArgs.SendArg) //Get the ID (big.ing) from the sender, and find via getall func which files he should receive
		}
	} else if sendArgs.GetIdentifier {
		receiveArgs.ReplyArgs = n.Flags.UserID
	}
	return nil
}

/*
Add the file to the node's bucket by associating the file with a key
in the bucket and save file's data, and save the file content to the disk
*/
func (n *Node) putFile(file File) {
	//Generate a key for the file based on its id
	key := file.ID.String()

	n.Bucket[key] = append(n.Bucket[key], file.FileName)

	CheckError(n.saveToFile(key, file.FileName, file.Content), "Savefile")
}

/*
saveToFile creates directories for the files to be saved in on disk. Saves the file on the correct directory.
*/

func (n *Node) saveToFile(IdKey string, filename string, content []byte) error {
	// Create or open file to write
	BucketDirectory := "bucket" + n.Id.String()

	// Create directory and its subdirectory if it doesn't exist yet
	err := os.MkdirAll(BucketDirectory, os.ModePerm)
	CheckError(err, "mkdirall bucketDirectory in Savetofile")

	KeyDirectory := fmt.Sprintf("%s/%s", BucketDirectory, IdKey)

	err = os.MkdirAll(KeyDirectory, os.ModePerm)
	CheckError(err, "mkdirall, KeyDirectory, in Savetofile")

	filePath := fmt.Sprintf("%s/%s", KeyDirectory, filename)

	err = ioutil.WriteFile(filePath, content, 0700)
	if CheckError(err, "write file") {
		return err
	}
	if Debugging {
		fmt.Printf("Innehållet har sparats i filen: %s\n", filename)
	}
	return nil
}

/*
putAll receives a map of key/value pairs, and adds all of its contents to the local bucket of key/value pairs
When a node is about to go down in response to a quit command, call put_all on its successor,
handing it the entire local bucket before shutting down.
*/
func (n *Node) putAll(received map[string][]File) {

	for key, files := range received {
		existingValues := n.Bucket[key]

		fileNames := make([]string, 0)

		for _, file := range files {
			fileNames = append(fileNames, file.FileName)

			err := n.saveToFile(file.ID.String(), file.FileName, file.Content)
			CheckError(err, "In YourFunctionName, during saveToFile")
		}

		existingValues = append(existingValues, fileNames...)
		n.Bucket[key] = existingValues
	}
}

/*
It receives an adress from our new predecessor.
It then calculates which IDs are between our new predecessor and our old one.
After that it extracts all of its files with those IDs that are between and
sends them to the adress belonging to the new predecessor.
Then it locally deletes the files that were sent
*/
func (n *Node) getAll(NewPredID *big.Int) map[string][]File {

	Filebucket := make(map[string][]File)

	OldPredID := new(big.Int)
	if n.Predecessor != "" {
		OldPredID = hashModulo(Hash(n.Predecessor), n.M2)

	} else {
		OldPredID = &n.Id
	}

	KeyBigInt := new(big.Int)

	for key, fileNames := range n.Bucket {
		KeyBigInt, _ := KeyBigInt.SetString(key, 10)

		if between(OldPredID, KeyBigInt, NewPredID, true) {
			if Debugging {
				fmt.Printf("Is between\n")
				fmt.Printf("Katalog: %s\n", key)
			}

			for _, fileName := range fileNames {

				filepath := fmt.Sprintf("%s/%s/%s", "bucket"+n.Id.String(), key, fileName)
				content, readok := readFile(filepath)
				if readok {
					Key := new(big.Int)
					Key.SetString(key, 10)

					fmt.Printf("Appending Filestruct with ID %s, Filename %s \n", Key.String(), fileName)

					file := File{ID: *Key, FileName: fileName, Content: content}
					Filebucket[key] = append(Filebucket[key], file)
				} else {
					fmt.Printf("No such file on disk in getAll\n")
					continue
				}
			}
			//Delete the files in that key directory.
			n.deleteDirectory("bucket" + n.Id.String() + "/" + key)
			delete(n.Bucket, key) //Remove the key from the local bucket. (OBS NOT FILE ON DISK, JUST THE BUCKET MAP)
		} else {
			continue
		}

	}

	return Filebucket
	// takes the address of a new node that is between you and your predecessor. It should gather all keys that belong to that new node
	//(use your between function to determine this) into a new map, and it should
	//also remove them from your bucket. You can loop through all the values in a map like this:
	//When joining an existing ring, issue a get_all request to your new successor once the join has succeeded, i.e., as soon as you know your successor.
}

/*
Deletes a folder with all of its files
*/
func (n *Node) deleteDirectory(folderPath string) {
	err := os.RemoveAll(folderPath)
	if CheckError(err, "In deleteDirectory") {
		return
	}
	fmt.Printf("Folder %s and its content is deleted \n", folderPath)
}

/*
Finds and returns the successor of a node if the node is between itself and its succsesor
otherwise it returns the adress of the closest preceding node
*/
func (n *Node) findSuccessor(id big.Int) (bool, string) {

	//hash the address of the first element in successor
	sucId := hashModulo(Hash(n.Successors[0]), n.M2)

	if between(&n.Id, &id, sucId, true) {
		return true, n.Successors[0]
	} else {
		//If closestPrecedingNode is curId then we return true and the adress
		closestAddress := n.closestPrecedingNode(id)
		if closestAddress == n.Address {
			return true, closestAddress
		} else {
			//If it is another node then we return it and false
			return false, closestAddress
		}
	}
}

/*
Returns the address of the closes preceding node. Checks the closest candidate form the fingertable
and the successor table and returns the closest of the two of them.
*/
func (n *Node) closestPrecedingNode(id big.Int) string {
	fingerTableChoice := ""
	SuccTableChoice := ""

	for i := len(n.FingerTable) - 1; i >= 0; i-- {
		if n.FingerTable[i] != "" {
			fingerTableValueHash := hashModulo(Hash(n.FingerTable[i]), n.M2)

			if fingerTableValueHash != nil && between(&n.Id, fingerTableValueHash, &id, false) {

				fingerTableChoice = n.FingerTable[i]
				break
			}
		}
	}

	for i := len(n.Successors) - 1; i >= 0; i-- {
		if n.Successors[i] != "" {
			SuccessorsTableValueHash := hashModulo(Hash(n.Successors[i]), n.M2)
			if SuccessorsTableValueHash != nil && between(&n.Id, SuccessorsTableValueHash, &id, true) { //Including the edge here for faster "Lookup"
				SuccTableChoice = n.Successors[i]
				break
			}
		}
	}

	if fingerTableChoice == "" && SuccTableChoice == "" {
		return n.Address // If no nearby preceding node is found, return the current node.
	} else {
		FingerDistance := n.CalculateDistance(*hashModulo(Hash(fingerTableChoice), n.M2), id)
		SuccDistance := n.CalculateDistance(*hashModulo(Hash(SuccTableChoice), n.M2), id)

		if FingerDistance.Cmp(big.NewInt(0)) == 0 {
			return fingerTableChoice
		} else if SuccDistance.Cmp(big.NewInt(0)) == 0 {
			return SuccTableChoice
		}
		if FingerDistance.Cmp(SuccDistance) <= 0 {
			return fingerTableChoice
		} else {
			return SuccTableChoice
		}
	}
}

/*
Calculate the distance between two positions in the ring
*/
func (n *Node) CalculateDistance(candidat, target big.Int) *big.Int {
	distance := new(big.Int)

	if candidat.Cmp(&target) < 0 {
		// If candidate is less than the target on the ring
		distance.Sub(&target, &candidat)
	}
	if candidat.Cmp(&target) > 0 {
		// If candidate is greater than the target on the ring
		target.Add(&target, &n.M2) //Add the ring size to the target to maintaine the structure
		distance.Sub(&target, &candidat)
	}
	return distance
}

/*
Finds and returns the successor of a given node by id, starting the search at node "start" and stops if maxSteps is reached.
Runs iteratively until a valid successors is found
*/
func (n *Node) find(id big.Int, start string, maxSteps int) (bool, string) {
	found, nextNode := false, start
	i := 0

	SenderArgs := SendArgs{GetSuccessorRequest: true, SendArg: id} //The argument to send to the node we are joining is the current nodes address.
	ReceiveArgs := ReceiveArgs{}

	for !found && i < maxSteps {
		ok := n.call("Node.CallHandler", &SenderArgs, &ReceiveArgs, nextNode)
		if ok {
			found = ReceiveArgs.FindSuccessorAnswer.IsSuccessor
			nextNode = ReceiveArgs.FindSuccessorAnswer.Address
			i++

		} else {
			fmt.Printf("Error during call in find\n")
			return false, ""
		}

	}
	if found {
		//fmt.Printf("Successor %s found for node %s \n", nextNode, start)
		return true, nextNode
	} else {
		fmt.Println("Error: Node address not found.")
		return false, ""
	}

}

var two = big.NewInt(2)

/*
Calculates the jump-length which is equal to id + 2^(fingerentry-1). Where fingerentry varies from 0 - len(Fingertable) -1
Returns a big-int value which is the value of (id + 2^(fingerentry-1)) % ringsize.
*/
func (n *Node) jump(id big.Int, fingerentry int) *big.Int {

	fingerentryminus1 := big.NewInt(int64(fingerentry) - 1) //next -1
	jump := new(big.Int).Exp(two, fingerentryminus1, nil)   //2^(next -1)
	sum := new(big.Int).Add(&id, jump)                      //  id + 2^(next -1)

	result := new(big.Int).Mod(sum, &n.M2) // sum % M2  (RINGSIZE)

	if Debugging {
		fmt.Printf("id: %s, (next^2): %s,Sum: = %s ::: Sum(mod)m^2 = : %s\n", id.String(), jump.String(), sum.String(), result.String())
	}
	return result
}

const keySize = sha1.Size * 8

/*
Get the identifiere from the Node on the given adress
By sending a request to the node to get its identifier
*/
func (n *Node) GetIdentifier(Address string) string {

	SenderArgsNotify := SendArgs{GetIdentifier: true}
	ReceiveArgs := ReceiveArgs{}
	ok := n.call("Node.CallHandler", &SenderArgsNotify, &ReceiveArgs, Address)

	if ok {
		return ReceiveArgs.ReplyArgs
	} else {
		return ""
	}
}

/*
Prints the all important details of a node including the finger table entries, successor list, predecessor and bucket content
*/
func (n *Node) PrintDetails() {
	fmt.Println("********-Node Details:-********")
	fmt.Printf("Id: %s, Identifier: %s, Address: %s\n", n.Id.String(), n.Flags.UserID, n.Address)
	fmt.Printf("Finger Table: Size: %d\n", len(n.FingerTable))
	for i, entry := range n.FingerTable {
		if entry != "" {
			fmt.Printf("  -Entry %d:(Id %s + %d) Identifier: %s, ID: %s, Address: %s\n", i, n.Id.String(), int(math.Pow(2, float64(i))), n.GetIdentifier(entry), hashModulo(Hash(entry), n.M2).String(), entry)
		}
	}
	if n.Predecessor != "" {
		fmt.Printf("Predecessor: Identifier: %s, ID: %s, Address: %s\n", n.GetIdentifier(n.Predecessor), hashModulo(Hash(n.Predecessor), n.M2).String(), n.Predecessor)
	} else {
		fmt.Printf("Predecessor: Identifier: , ID: , Address: \n")
	}
	fmt.Printf("Successors: Size: %d\n", len(n.Successors))
	for i, successor := range n.Successors {
		if successor != "" {
			fmt.Printf("  -Entry %d: Identifier: %s, ID: %s, Address: %s\n", i, n.GetIdentifier(successor), hashModulo(Hash(successor), n.M2).String(), successor)
		}
	}
	if Debugging {
		// Output for Flags-struct
		fmt.Printf("Flags struct: %+v\n", n.Flags)

		// Output for intent, M2 and M
		fmt.Printf(" M2: %s, M: %d\n", n.M2.String(), n.M)
	}
	fmt.Println("Bucket:")
	for key, value := range n.Bucket {
		fmt.Printf("  Key: %s, Value: %s\n", key, value)
	}

	fmt.Println("********-END Node Details:-********")
}
