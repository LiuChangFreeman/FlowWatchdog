request = function()
    key = math.random(1, 10000000)
    path = "/?key=" .. key
    return wrk.format(nil, path)
end

local threads = {}

function setup(thread)
    table.insert(threads, thread)
 end

function init(args)     
    responses= 0    
end    

function response(status, headers, body)       
    if status == 200 then        
        if string.find(body,'"value": ') then 
            responses= responses + 1     
        end    
    end    
end    

function done(summary, latency, requests)    
    local total_responses = 0    
    for _, thread in ipairs(threads) do        
        local responses = thread:get("responses")    
        total_responses = total_responses + responses    
    end    
    local msg = "success_response: %s"    
    print(msg:format(total_responses))    
end 