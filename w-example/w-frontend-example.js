/////////// 1- Configuration: When creating it, you usually provide the URLs of your STUN/TURN servers: 
const configuration = {
	iceServers: [
	  { urls: 'stun:stun.l.google.com:19302' }, // Example public STUN
	  // { urls: 'turn:your-turn-server.com', username: 'user', credential: 'password' } // Example TURN
	]
      };


/////////// 2- The RTCPeerConnection API , create object: peerConnection
const peerConnection = new RTCPeerConnection(configuration);


/////////// 3- Get User Media: Before making a call, you need access to the user's camera and microphone:
let localStream;
try {
  localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
  	// Now you have the user's audio/video in 'localStream'
  	// You can display this local video in an HTML <video> element
} catch (error) {
  console.error('Error accessing media devices.', error);
}


/////////// 4- Add Media to Connection: Add the tracks (audio, video) from your localStream to the RTCPeerConnection so they can be sent to the peer:
localStream.getTracks().forEach(track => {
	peerConnection.addTrack(track, localStream);
      });


/////////// 5- Create Offer (Caller): The user initiating the call (Caller) creates an SDP offer:
const offer = await peerConnection.createOffer();
await peerConnection.setLocalDescription(offer); // Tell PC about the offer

	// NOW: Send 'offer' to the other user via your Signaling Server (e.g., WebSocket)
	// signalingServer.send({ type: 'offer', sdp: offer });


/////////// 6- Receive Offer & Create Answer (Callee):

	//The user receiving the call (Callee) gets the offer via the signaling server.
	//They set it as the remote description:
await peerConnection.setRemoteDescription(new RTCSessionDescription(receivedOffer.sdp));

	//The Callee then gets their own localStream (using getUserMedia) and adds their tracks (addTrack).
	//The Callee creates an SDP answer:

const answer = await peerConnection.createAnswer();
await peerConnection.setLocalDescription(answer); // Tell PC about the answer

	// NOW: Send 'answer' back to the Caller via your Signaling Server
	// signalingServer.send({ type: 'answer', sdp: answer });


/////////// 7- Receive Answer (Caller): The Caller receives the answer via signaling and sets it as the remote description:
await peerConnection.setRemoteDescription(new RTCSessionDescription(receivedAnswer.sdp));


/////////////////////////////////////////////////////////////////////////////////////////////////////
/////////// a visual diagram for this part:

// sequenceDiagram
//     participant Caller
//     participant SignalingServer
//     participant Callee

//     Note over Caller, Callee: Initial setup (getUserMedia, create RTCPeerConnection, addTrack) happens first or during this flow

//     Caller->>Caller: createOffer()
//     Caller->>Caller: setLocalDescription(offer)
//     Caller->>SignalingServer: Send Offer (SDP)
//     SignalingServer->>Callee: Forward Offer (SDP)

//     Callee->>Callee: setRemoteDescription(offer)
//     Callee->>Callee: createAnswer()
//     Callee->>Callee: setLocalDescription(answer)
//     Callee->>SignalingServer: Send Answer (SDP)
//     SignalingServer->>Caller: Forward Answer (SDP)

//     Caller->>Caller: setRemoteDescription(answer)

//     Note over Caller, Callee: SDP negotiation complete. ICE Candidate exchange (next step) establishes the actual media path.
/////////////////////////////////////////////////////////////////////////////////////////////////////



/////////// 8- Exchanging ICE Candidates
	//You need to listen for this event and send each candidate to the other peer using your Signaling Server.
	
	// *** On BOTH Caller and Callee ***

	peerConnection.onicecandidate = event => {
		if (event.candidate) {
		  // Found a new candidate! Send it to the other peer via signaling.
		  console.log('Sending ICE candidate:', event.candidate);
		  // signalingServer.send({ type: 'candidate', candidate: event.candidate });
		} else {
		  // All candidates have been gathered.
		  console.log('ICE Candidate gathering complete.');
		}
	      };


/////////// 9- When one peer receives a candidate message from the other via the signaling server, they need to add it to their local RTCPeerConnection:

	// *** On BOTH Caller and Callee (when receiving a candidate message) ***

	// Assume 'receivedCandidate' contains the candidate object from the signaling message
	if (receivedCandidate.candidate) {
		try {
		await peerConnection.addIceCandidate(new RTCIceCandidate(receivedCandidate.candidate));
		console.log('Added received ICE candidate');
		} catch (error) {
		console.error('Error adding received ICE candidate', error);
		}
	}


/////////// 10- Receiving the Remote Media Stream

	// Once the Offer/Answer exchange is complete and ICE candidates have been successfully exchanged and tested, the RTCPeerConnections establish a connection.

	// When the remote peer starts sending their audio/video tracks (which they added using addTrack earlier), your local RTCPeerConnection triggers an ontrack event.

	// The event object contains the remote media stream. You need to grab this stream and display it in another HTML <video> element.


	// *** On BOTH Caller and Callee ***
	//in react we will use "useRef"
	const remoteVideo = document.getElementById('remoteVideo'); // Assuming you have <video id="remoteVideo"></video>

	peerConnection.ontrack = event => {
	console.log('Received remote track!');
	if (remoteVideo.srcObject !== event.streams[0]) {
	remoteVideo.srcObject = event.streams[0]; // Assign the stream to the video element
	console.log('Attached remote stream to video element.');
	}
	};

	// Key Point: You don't explicitly tell it when to start sending/receiving media. Once the connection is established and tracks have been added (addTrack), WebRTC handles sending them. The ontrack event tells you when data from the other side arrives.


	// Connection State: You can monitor the connection status:
	
	peerConnection.oniceconnectionstatechange = () => {
	console.log(`ICE Connection State: ${peerConnection.iceConnectionState}`);
	// Common states: 'new', 'checking', 'connected', 'completed', 'disconnected', 'failed', 'closed'
	if (peerConnection.iceConnectionState === 'connected' || peerConnection.iceConnectionState === 'completed') {
	console.log('Peers connected!');
	// Call is active
	} else if (peerConnection.iceConnectionState === 'failed') {
	console.error('Connection failed, cleanup necessary.');
	// Handle failure
	} else if (peerConnection.iceConnectionState === 'disconnected' || peerConnection.iceConnectionState === 'closed') {
	console.log('Connection closed or disconnected.');
	// Handle call ending
	}
	};



//////////////////////////////////////////////////////////////////////////////////////////////////////////
/////////// a visual diagram for an example: alice call bob, the logic call-flow:

// sequenceDiagram
//     participant Alice
//     participant SignalingServer
//     participant Bob
//     participant Alice_PC as Alice RTCPeerConnection
//     participant Bob_PC as Bob RTCPeerConnection

//     Alice->>Alice: User clicks "Call Bob"
//     Alice->>Alice: getUserMedia() -> localStreamA
//     Alice->>Alice_PC: new RTCPeerConnection()
//     Alice_PC->>Alice_PC: Add localStreamA tracks
//     Alice_PC->>Alice_PC: Setup onicecandidate, ontrack
//     Alice_PC->>Alice_PC: createOffer() -> offer
//     Alice_PC->>Alice_PC: setLocalDescription(offer)
//     Alice->>SignalingServer: Send Offer (to Bob) { sdp: offer }

//     SignalingServer->>Bob: Forward Offer (from Alice) { sdp: offer }

//     Bob->>Bob_PC: new RTCPeerConnection()
//     Bob_PC->>Bob_PC: Setup onicecandidate, ontrack
//     Bob_PC->>Bob_PC: setRemoteDescription(offer)
//     Bob->>Bob: getUserMedia() -> localStreamB
//     Bob_PC->>Bob_PC: Add localStreamB tracks
//     Bob_PC->>Bob_PC: createAnswer() -> answer
//     Bob_PC->>Bob_PC: setLocalDescription(answer)
//     Bob->>SignalingServer: Send Answer (to Alice) { sdp: answer }

//     SignalingServer->>Alice: Forward Answer (from Bob) { sdp: answer }
//     Alice_PC->>Alice_PC: setRemoteDescription(answer)

//     Note over Alice, Bob: --- SDP Offer/Answer Complete ---

//     par ICE Candidate Exchange
//         Alice_PC->>Alice: onicecandidate (candidateA)
//         Alice->>SignalingServer: Send Candidate (to Bob) { candidate: candidateA }
//         SignalingServer->>Bob: Forward Candidate (from Alice) { candidate: candidateA }
//         Bob_PC->>Bob_PC: addIceCandidate(candidateA)
//     and
//         Bob_PC->>Bob: onicecandidate (candidateB)
//         Bob->>SignalingServer: Send Candidate (to Alice) { candidate: candidateB }
//         SignalingServer->>Alice: Forward Candidate (from Bob) { candidate: candidateB }
//         Alice_PC->>Alice_PC: addIceCandidate(candidateB)
//     end
//     Note over Alice, Bob: ...(ICE Candidates exchanged as needed)...

//     Alice_PC->>Alice_PC: iceConnectionState changes to 'connected'/'completed'
//     Bob_PC->>Bob_PC: iceConnectionState changes to 'connected'/'completed'

//     Note over Alice, Bob: --- P2P Connection Established ---

//     par Media Streaming
//         Bob_PC->>Alice_PC: Sends media track(s)
//         Alice_PC->>Alice: ontrack (remoteStreamB)
//         Alice->>Alice: Display remoteStreamB in <video>
//     and
//         Alice_PC->>Bob_PC: Sends media track(s)
//         Bob_PC->>Bob: ontrack (remoteStreamA)
//         Bob->>Bob: Display remoteStreamA in <video>
//     end

////////////////////////////////////////////////////////////////////////////////////////


/////////// 11- Ending the Call

	// - User Action: One user (e.g., Alice) clicks a "Hang Up" button.


	// - Close PeerConnection: Call peerConnection.close() on Alice's side.
		// This stops media transmission and releases camera/microphone resources associated with that connection.
		// It triggers state changes (oniceconnectionstatechange to 'closed').


	// - Stop Local Media: Explicitly stop the local media tracks obtained via getUserMedia to turn off the camera/mic completely:
		// use: localStream.getTracks().forEach(track => track.stop());
			// Also clear the local video element srcObject if needed:
			// use: localVideo.srcObject = null;


	// - (Optional but Recommended) Signal Hang Up: Send a message via the Signaling Server to the other user (Bob) indicating the call has ended (e.g., { type: 'hangup', target: 'bob_id' }).


	// - Other User (Bob) Receives Hangup Signal:
		// When Bob receives the 'hangup' signal (or detects the peerConnection state change to 'disconnected'/'closed'/'failed'):
		// Bob should also call peerConnection.close() on his side.
		// Bob should stop his local media tracks (localStream.getTracks().forEach(track => track.stop());).
		// Clean up UI (e.g., hide video elements, show "Call Ended" message).


	// - Why signal hangup? Relying solely on oniceconnectionstatechange might have delays or ambiguity. An explicit signal is cleaner.









	import React, { useState, useEffect, useRef, useCallback } from 'react';

	// Assume you have a way to send/receive signaling messages
	// (e.g., using a WebSocket hook or context)
	// function sendSignalingMessage(message) { /* ... */ }
	// function useSignaling(onMessageCallback) { /* returns send function */ }
	
	const STUN_SERVER = 'stun:stun.l.google.com:19302';
	
	function WebRtcCallComponent({ signaling, targetUserId, isCaller }) {
	  const [localStream, setLocalStream] = useState(null);
	  const [remoteStream, setRemoteStream] = useState(null);
	  const [callState, setCallState] = useState('idle'); // idle, calling, receiving, connected
	  const peerConnectionRef = useRef(null); // To hold the RTCPeerConnection instance
	  const localVideoRef = useRef(null);   // Ref for local <video> element
	  const remoteVideoRef = useRef(null);  // Ref for remote <video> element
	
	  // --- 1. Initialize and Cleanup ---
	  useEffect(() => {
	    // Cleanup function runs when component unmounts or dependencies change forcing re-run
	    return () => {
	      hangUp(); // Ensure cleanup happens
	    };
	  }, []); // Empty dependency array means this runs once on mount for cleanup setup
	
	  // --- 2. Signaling Message Handler ---
	  const handleSignalingMessage = useCallback(async (message) => {
	    if (!message || message.target !== /* myUserId */ ) return; // Ignore irrelevant messages
	
	    const pc = peerConnectionRef.current;
	
	    try {
	      if (message.type === 'offer' && !isCaller) {
		console.log('Received OFFER');
		await initializePeerConnection(); // Ensure PC is ready
		await pc.setRemoteDescription(new RTCSessionDescription(message.sdp));
		await createAndSendAnswer();
		setCallState('receiving'); // Update state
	      } else if (message.type === 'answer' && isCaller) {
		console.log('Received ANSWER');
		if (pc.signalingState !== 'stable') { // Prevent race conditions
		   await pc.setRemoteDescription(new RTCSessionDescription(message.sdp));
		}
	      } else if (message.type === 'candidate') {
		console.log('Received ICE CANDIDATE');
		if (pc && message.candidate) {
		   // Add candidate only after remote description is set
		   if (pc.remoteDescription) {
		       await pc.addIceCandidate(new RTCIceCandidate(message.candidate));
		   } else {
		       // Queue candidate if remote description isn't set yet (less common with modern WebRTC)
		       console.warn("Queuing ICE candidate as remote description is not set yet.");
		       // Add queuing logic if necessary based on testing
		   }
		}
	      } else if (message.type === 'hangup') {
		console.log('Received HANGUP');
		hangUp();
	      }
	    } catch (error) {
	      console.error("Error handling signaling message:", error);
	    }
	  }, [isCaller, /* myUserId */, createAndSendAnswer, hangUp, initializePeerConnection]); // Include dependencies
	
	  // Subscribe to signaling messages (adapt based on your signaling implementation)
	  // useEffect(() => {
	  //   const unsubscribe = signaling.subscribe(handleSignalingMessage);
	  //   return unsubscribe;
	  // }, [handleSignalingMessage, signaling]);
	
	
	  // --- 3. Core WebRTC Functions ---
	  const initializePeerConnection = useCallback(async () => {
	    if (peerConnectionRef.current) return; // Already initialized
	
	    try {
	      // A. Get Local Media
	      const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
	      setLocalStream(stream);
	      if (localVideoRef.current) {
		localVideoRef.current.srcObject = stream; // Display local video immediately
	      }
	
	      // B. Create PeerConnection
	      const pc = new RTCPeerConnection({ iceServers: [{ urls: STUN_SERVER }] });
	      peerConnectionRef.current = pc;
	
	      // C. Add Local Tracks
	      stream.getTracks().forEach(track => pc.addTrack(track, stream));
	
	      // D. Setup Event Listeners
	      pc.onicecandidate = (event) => {
		if (event.candidate) {
		  console.log('Sending ICE CANDIDATE');
		  signaling.send({ type: 'candidate', target: targetUserId, candidate: event.candidate });
		}
	      };
	
	      pc.ontrack = (event) => {
		console.log('Received REMOTE TRACK');
		if (event.streams && event.streams[0]) {
		  setRemoteStream(event.streams[0]);
		  if (remoteVideoRef.current) {
		    remoteVideoRef.current.srcObject = event.streams[0];
		  }
		}
	      };
	
	      pc.oniceconnectionstatechange = () => {
		console.log(`ICE Connection State: ${pc.iceConnectionState}`);
		if (pc.iceConnectionState === 'connected' || pc.iceConnectionState === 'completed') {
		  setCallState('connected');
		} else if (['disconnected', 'failed', 'closed'].includes(pc.iceConnectionState)) {
		   hangUp(); // Clean up if connection drops unexpectedly
		}
		// Update UI based on state if needed
	      };
	
	    } catch (error) {
	      console.error("Error initializing PeerConnection or getting media:", error);
	      // Handle errors appropriately (e.g., show message to user)
	    }
	  }, [targetUserId, signaling.send, hangUp]); // Dependencies
	
	  const createAndSendOffer = useCallback(async () => {
	    if (!peerConnectionRef.current) return;
	    try {
	      const offer = await peerConnectionRef.current.createOffer();
	      await peerConnectionRef.current.setLocalDescription(offer);
	      console.log('Sending OFFER');
	      signaling.send({ type: 'offer', target: targetUserId, sdp: offer });
	      setCallState('calling');
	    } catch (error) {
	      console.error("Error creating/sending offer:", error);
	    }
	  }, [targetUserId, signaling.send]);
	
	  const createAndSendAnswer = useCallback(async () => {
	     if (!peerConnectionRef.current) return;
	     try {
	       const answer = await peerConnectionRef.current.createAnswer();
	       await peerConnectionRef.current.setLocalDescription(answer);
	       console.log('Sending ANSWER');
	       signaling.send({ type: 'answer', target: targetUserId, sdp: answer });
	       // State might already be 'receiving' or potentially 'connected' soon
	     } catch (error) {
	       console.error("Error creating/sending answer:", error);
	     }
	  }, [targetUserId, signaling.send]);
	
	
	  // --- 4. Call Control Functions ---
	  const startCall = useCallback(async () => {
	    if (callState === 'idle') {
	      await initializePeerConnection();
	      await createAndSendOffer();
	    }
	  }, [callState, initializePeerConnection, createAndSendOffer]);
	
	  const hangUp = useCallback(() => {
	    const pc = peerConnectionRef.current;
	    if (pc) {
	      pc.onicecandidate = null;
	      pc.ontrack = null;
	      pc.oniceconnectionstatechange = null;
	      pc.close();
	      peerConnectionRef.current = null;
	    }
	
	    if (localStream) {
	      localStream.getTracks().forEach(track => track.stop());
	      setLocalStream(null);
	    }
	    setRemoteStream(null); // Clear remote stream state
	
	    if(callState !== 'idle' && callState !== 'ended') {
		signaling.send({ type: 'hangup', target: targetUserId }); // Notify other peer
	    }
	
	    setCallState('ended'); // Or 'idle' if you want to allow immediate recall
	
	    // Clear video elements
	    if (localVideoRef.current) localVideoRef.current.srcObject = null;
	    if (remoteVideoRef.current) remoteVideoRef.current.srcObject = null;
	
	    console.log('Call hung up and resources released.');
	
	  }, [localStream, targetUserId, signaling.send, callState]);
	
	
	  // --- 5. Render UI ---
	  return (
	    <div>
	      <h2>WebRTC Call</h2>
	      <p>Call State: {callState}</p>
	
	      <div>
		<h3>Local Video</h3>
		<video ref={localVideoRef} autoPlay playsInline muted style={{ width: '300px' }} />
	      </div>
	      <div>
		<h3>Remote Video</h3>
		<video ref={remoteVideoRef} autoPlay playsInline style={{ width: '300px' }} />
	      </div>
	
	      <div>
		{callState === 'idle' && isCaller && (
		  <button onClick={startCall}>Call {targetUserId}</button>
		)}
		{/* Add Answer button if state is 'receiving' */}
		{/* {callState === 'receiving' && <button onClick={answerCall}>Answer</button>} */}
		{(callState === 'calling' || callState === 'receiving' || callState === 'connected') && (
		  <button onClick={hangUp}>Hang Up</button>
		)}
		 {callState === 'ended' && <p>Call Ended</p>}
	      </div>
	
	       {/* Integrate your actual signaling mechanism here */}
	       {/* Example: This component might receive 'signaling' prop containing send/subscribe methods */}
	    </div>
	  );
	}
	
	export default WebRtcCallComponent;



//////////////////////////////////////////////////////////////////////////////

// src/store.js
import { create } from 'zustand';

// Simple logging middleware
const log = (config) => (set, get, api) => config(
  (...args) => {
    console.log("applying", args); // Log before state change
    set(...args);
    console.log("new state", get()); // Log after state change
  },
  get,
  api
);

const useCounterStore = create(log((set) => ({ // <-- Wrap the definition with log()
  count: 0,
  increment: () => set((state) => ({ count: state.count + 1 })),
  decrement: () => set((state) => ({ count: state.count - 1 })),
  incrementAsync: async () => {
    await new Promise((resolve) => setTimeout(resolve, 1000));
    set((state) => ({ count: state.count + 1 }));
  },
})));

export default useCounterStore;


//////////////////////////////////////////////////////////////////////////////

	
// src/CounterComponent.js
// ... (imports remain the same, including shallow if you kept it)

function CounterComponent() {
	const { count, increment, decrement, incrementAsync } = useCounterStore(
	  (state) => ({
	    count: state.count,
	    increment: state.increment,
	    decrement: state.decrement,
	    incrementAsync: state.incrementAsync, // <-- Get async action
	  }),
	  shallow
	);
      
	return (
	  <div>
	    <p>Count: {count}</p>
	    <button onClick={increment}>Increment</button>
	    <button onClick={decrement}>Decrement</button>
	    <button onClick={incrementAsync}>Increment Async (1s delay)</button> {/* <-- Add button */}
	  </div>
	);
      }
      
      export default CounterComponent;