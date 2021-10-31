from uuid import uuid4
import json
from time import sleep
import asyncio
import websockets

key = "293faecad3499bdd836090ffc2a72693954be4c842c5728ae9a2148cf802a9359fe27a6ad42af696d3d3008557c8953f93c3681764edad05aa237932e1cc9d45678e7386f625c8d119595e67ff404312ddcfa642f4a2816fc838dc2ec3924fa044c92a7e0cb2e493519ec18a6d4879a9e091312c58f2bc472aa52dcca955799b"

### message crafting functions

def craftMessage(headerStr, bodyDict): 

    header = headerStr + (" " * (32 - len(headerStr)))
    body = json.dumps(bodyDict)
    
    return header + body

def craftInit(clientId):

    header = "HELO"
    bodyDict = {
        "clientId" : clientId,
        "key" : key
    }
    
    return craftMessage(header, bodyDict)

def craftCreate():

    header = "GAME_REQ"
    bodyDict = {
        "action" : "CREATE"
    }
    
    return craftMessage(header, bodyDict)

def craftJoin(gameId):

    header = "GAME_REQ"
    bodyDict = {
        "action" : "JOIN",
        "gameId" : gameId
    }
    
    return craftMessage(header, bodyDict)

def craftLeave(gameId):

    header = "GAME_REQ"
    bodyDict = {
        "action" : "LEAVE",
        "gameId" : gameId
    }
    
    return craftMessage(header, bodyDict)
    
def craftQuestionSelect(gameId, category, pointValue):

    header = "GAMEPLAY"
    bodyDict = {
        "req" : "QUESTION_SELECT",
        "gameId" : gameId,
        "category" : category,
        "pointValue" : pointValue
    }
    
    return craftMessage(header, bodyDict)

def craftBuzz(gameId, delay, expired):

    header = "GAMEPLAY"
    bodyDict = {
        "req" : "BUZZ",
        "gameId" : gameId,
        "delay" : delay,
        "expired" : expired
    }
    
    return craftMessage(header, bodyDict)

def craftAnswer(gameId, answerIndex):

    header = "GAMEPLAY"
    bodyDict = {
        "req" : "ANSWER",
        "gameId" : gameId,
        "answerIndex" : answerIndex
    }
    
    return craftMessage(header, bodyDict)

def craftWheelSpin(gameId, spinValue):
    
    header = "GAMEPLAY"
    bodyDict = {
        "req" : "WHEEL_SPIN",
        "gameId" : gameId,
        "spinValue" : spinValue
    }
    
    return craftMessage(header, bodyDict)

def parseIncoming(message):

    header = message[:32]
    print("header = {}".format(header))
    body = message[32:]
    print("body = {}".format(body))
    

async def clientMain():

    def handleCreate():
        return craftCreate()
    
    def handleJoin():
        gameId = input("gameId = ")
        return craftJoin(gameId)
    
    def handleLeave():
        gameId = input("gameId = ")
        return craftLeave(gameId)
    
    def handleSpinWheel():
        gameId = input("gameId = ")
        spinValue = float(input("spinValue = "))
        return craftWheelSpin(gameId, spinValue)
    
    def handleQuestionSelect():
        gameId = input("gameId = ")
        category = input("category = ")
        pointValue = int(input("pointValue = "), 10)
        return craftQuestionSelect(gameId, category, pointValue)
    
    def handleBuzz():
        gameId = input("gameId = ")
        delay = int(input("delay = "), 10)
        expired = input("expired = ")
        return craftBuzz(gameId, delay, expired)
    
    def handleAnswer():
        gameId = input("gameId = ")
        answerIndex = input("answerIndex = ")
        return craftAnswer(gameId, answerIndex)

    commandMap = {
        "1" : handleCreate,
        "2" : handleJoin,
        "3" : handleLeave,
        "4" : handleSpinWheel,
        "5" : handleQuestionSelect,
        "6" : handleBuzz,
        "7" : handleAnswer
    }

    clientId = str(uuid4())
    initMessage = craftInit(clientId)

    async with websockets.connect("ws://localhost:8080") as websocket:
        await websocket.send(initMessage)
        data = await websocket.recv()
        parseIncoming(data)
        
        while True:
            print("Select command (1 - 8):")
            print("\t(1) create game")
            print("\t(2) join game")
            print("\t(3) leave game")
            print("\t(4) spin wheel (gameplay)")
            print("\t(5) select question (gameplay)")
            print("\t(6) buzz (gameplay)")
            print("\t(7) answer question (gameplay)")
            print("\t(8) exit")
            command = (input()).strip()
            
            if command == "8":
                break
            
            message = commandMap[command]()
            
            await websocket.send(message)
            data = await websocket.recv()
            parseIncoming(data)
    
def main():
    clientId = asyncio.get_event_loop().run_until_complete(clientMain())
    print(clientId)

if __name__ == "__main__":
    main()