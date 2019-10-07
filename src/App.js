import React from "react";
import $ from "jquery";
import auth0 from "auth0-js";
import "./index.css";
import axios from "axios";
import "antd/dist/antd.css";
import { Editor } from "react-draft-wysiwyg";
import "react-draft-wysiwyg/dist/react-draft-wysiwyg.css";
import { Row, Upload, message, Button, Icon, Spin } from "antd";

let globalRootURL = "http://" + window.location.host;

const AUTH0_CLIENT_ID = "JCvzA0JNJ5KCNONawklcvWx2MT1s53eK";
const AUTH0_DOMAIN = "online-litfest.auth0.com";
const AUTH0_API_AUDIENCE = "https://online-litfest.auth0.com/api/v2/";
const AUTH0_CALLBACK_URL = globalRootURL;

const { Dragger } = Upload;

class App extends React.Component {
  constructor() {
    super();
    this.state = { loggedIn: null };
    this.renderBody = this.renderBody.bind(this);
    this.setup();
    this.parseHash();
    this.setState();
  }
  parseHash() {
    this.auth0 = new auth0.WebAuth({
      domain: AUTH0_DOMAIN,
      clientID: AUTH0_CLIENT_ID
    });
    this.auth0.parseHash((err, authResult) => {
      if (err) {
        return console.log(err);
      }
      if (
        authResult !== null &&
        authResult.accessToken !== null &&
        authResult.idToken !== null
      ) {
        localStorage.setItem("access_token", authResult.accessToken);
        localStorage.setItem("id_token", authResult.idToken);
        localStorage.setItem(
          "email",
          JSON.stringify(authResult.idTokenPayload)
        );
        window.location = window.location.href.substr(
          0,
          window.location.href.indexOf("")
        );
      }
    });
  }
  setup() {
    $.ajaxSetup({
      beforeSend: function(xhr) {
        if (localStorage.getItem("access_token")) {
          xhr.setRequestHeader(
            "Authorization",
            "Bearer " + localStorage.getItem("access_token")
          );
        }
      }
    });
  }
  setState() {
    let idToken = localStorage.getItem("id_token");
    if (idToken) {
      this.state.loggedIn = true;
    } else {
      this.state.loggedIn = false;
    }
  }
  // componentWillMount() {
  //   this.setup();
  //   this.parseHash();
  //   this.setState();
  // }
  renderBody() {
    if (this.state.loggedIn) return <LoggedIn />;
    else return <Home />;
  }

  render() {
    return this.state.loggedIn === undefined ? (
      <div class="fh5co-loader"></div>
    ) : (
      <this.renderBody />
    );
  }
}

class Home extends React.Component {
  constructor(props) {
    super(props);
    this.authenticate = this.authenticate.bind(this);
  }
  authenticate() {
    this.WebAuth = new auth0.WebAuth({
      domain: AUTH0_DOMAIN,
      clientID: AUTH0_CLIENT_ID,
      scope: "openid email",
      audience: AUTH0_API_AUDIENCE,
      responseType: "token id_token",
      redirectUri: AUTH0_CALLBACK_URL
    });
    this.WebAuth.authorize();
  }
  componentDidMount() {
    this.authenticate();
  }
  render() {
    return <div></div>;
  }
}

class LoggedIn extends React.Component {
  constructor() {
    super();
    this.state = {
      currentEvent: null
    };
    this.setEvent = this.setEvent.bind(this);
  }
  setEvent(eventName) {
    this.setState({ currentEvent: eventName });
  }
  render() {
    if (this.state.currentEvent == null) {
      return <EventChoose setEvent={this.setEvent} />;
    } else if (this.state.currentEvent == "Memeify") {
      return <Memeify />;
    }
  }
}

class EventChoose extends React.Component {
  render() {
    return (
      <div>
        <Row>
          <Button
            size="large"
            type="primary"
            onClick={e => {
              this.props.setEvent(e.target.textContent);
            }}
          >
            Memeify
          </Button>
          <Button size="large" type="primary">
            Sweetheart
          </Button>
          <Button size="large" type="primary">
            Two-line Goosebumps
          </Button>
          <Button size="large" type="primary">
            Fun Sized Fables
          </Button>
          <Button size="large" type="primary">
            Reviews
          </Button>
          <Button size="large" type="primary">
            Short Stories
          </Button>
          <Button size="large" type="primary">
            Poetry/Limericks
          </Button>
          <Button size="large" type="primary">
            Dear Me
          </Button>
          <Button size="large" type="primary">
            Jimmy Fallon Hashtag Thing
          </Button>
          <Button size="large" type="primary">
            Essay 1
          </Button>
          <Button size="large" type="primary">
            Essay 2
          </Button>
        </Row>
      </div>
    );
  }
}

class Memeify extends React.Component {
  constructor() {
    super();
    this.state = { uploads: null };
    this.beforeUpload = this.beforeUpload.bind(this);
  }
  beforeUpload(file) {
    const isJpgOrPng = file.type === "image/jpeg" || file.type === "image/png";
    if (!isJpgOrPng) {
      message.error("You can only upload JPG/PNG file!");
    }
    const isLt2M = file.size / 1024 / 1024 < 2;
    if (!isLt2M) {
      message.error("Image must smaller than 2MB!");
    }

    if (isJpgOrPng && isLt2M) {
      this.setState({ uploads: this.state.uploads + 1 });
    }

    return isJpgOrPng && isLt2M;
  }
  componentDidMount() {
    const headers = {
      headers: {
        "Content-Type": "application/json",
        authorization: "Bearer " + localStorage.getItem("access_token")
      }
    };
    let url =
      "http://localhost:8080/api/private/memeify/entries?id_token=" +
      localStorage.getItem("id_token");
    axios.get(url, headers).then(res => {
      this.setState({ uploads: res.data.entries });
    });
  }
  render() {
    const props = {
      name: "file",
      action:
        "http://localhost:8080/api/private/memeify/upload?id_token=" +
        localStorage.getItem("id_token"),
      headers: {
        authorization: "Bearer " + localStorage.getItem("access_token")
      },
      onChange(info) {
        if (info.file.status !== "uploading") {
          console.log(info.file, info.fileList);
        }
        if (info.file.status === "done") {
          message.success(`${info.file.name} file uploaded successfully`);
        } else if (info.file.status === "error") {
          message.error(`${info.file.name} file upload failed.`);
        }
      },
      beforeUpload: this.beforeUpload
    };
    if (this.state.uploads == null) {
      return <Spin size="large"></Spin>;
    } else if (this.state.uploads > 3) {
      return <p>you've already submitted 3 entries</p>;
    } else if (this.state.uploads <= 3) {
      return (
        <Dragger {...props}>
          <p className="ant-upload-drag-icon">
            <Icon type="upload" />
          </p>
          <p className="ant-upload-text">
            Upload your meme as a jpg or png here.
          </p>
          <p className="ant-upload-hint">
            File size should be less than 2 MB. You can only upload a total of 3
            memes. Once uploaded, memes cannot be deleted.
          </p>
        </Dragger>
      );
    }
  }
}

export default App;
