export { template };

let template = {
    "connection-row": `
        <div class="row conn">
            <div class="col">
                <div class="form-check form-switch form-control-lg">
                    <input class="form-check-input conn-checkbox" id="conn-checkbox" type="checkbox" data-conn-id="">
                    <label class="row conn-label" for="connection-row">
                        <h2 class="label-name"></h2>
                        <h5 class="label-info">
                            <span class="addr"></span>
                            <span class="user"></span>
                        </h5>
                        <div class="conn-info">
                            <i class="bi bi-arrow-down down"></i> <span class="down-speed"></span>
                            <i class="bi bi-arrow-up up"></i> <span class="up-speed"></span>
                        </div>
                    </label>                                                           
                </div>                                
            </div>            
            <div class="col conn-edit">
                <span class="btn btn-outline-light btn-lg" role="button">
                    <i class="bi bi-pen"></i>
                </span>
            </div>
        </div>`,

    "journal-row": `
        <div class="row">
            <div class="col">
            <span class="time"></span>
            <span class="message"></span>
            </div>
        </div>`
};